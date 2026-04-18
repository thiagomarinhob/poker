package game

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRoomAutoFoldAfterTimeout(t *testing.T) {
	room, err := NewRoom(2, Blinds{Small: 10, Big: 20}, 60*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go room.Run(ctx)

	mustSendCmd(t, room, SitDown{PlayerID: "alice", Seat: 0, BuyIn: 200})
	mustSendCmd(t, room, SitDown{PlayerID: "bob", Seat: 1, BuyIn: 200})

	var complete HandComplete
	gotComplete := false
	deadline := time.After(2 * time.Second)

	for !gotComplete {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for HandComplete")
		case ev := <-room.Out:
			switch e := ev.(type) {
			case HandComplete:
				complete = e
				gotComplete = true
			}
		}
	}

	if len(complete.Winners) != 1 {
		t.Fatalf("expected one winner, got %+v", complete.Winners)
	}
	if complete.Winners[0] != "bob" {
		t.Fatalf("expected bob to win by timeout fold, got %+v", complete.Winners)
	}
}

func TestRoomConcurrentPlayersActing(t *testing.T) {
	room, err := NewRoom(3, Blinds{Small: 5, Big: 10}, 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go room.Run(ctx)

	mustSendCmd(t, room, SitDown{PlayerID: "alice", Seat: 0, BuyIn: 300})
	mustSendCmd(t, room, SitDown{PlayerID: "bob", Seat: 1, BuyIn: 300})
	mustSendCmd(t, room, SitDown{PlayerID: "carol", Seat: 2, BuyIn: 300, SitOut: true})

	deadline := time.After(4 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting hand complete")
		case ev := <-room.Out:
			if req, ok := ev.(ActionRequired); ok {
				go func(req ActionRequired) {
					actionType := Call
					if req.CanCheck {
						actionType = Check
					}
					_ = sendCmd(room, PlayerAction{
						PlayerID: req.PlayerID,
						Type:     actionType,
					})
				}(req)
				continue
			}
			if complete, ok := ev.(HandComplete); ok {
				if len(complete.Winners) == 0 {
					t.Fatal("expected at least one winner")
				}
				seatCarol := room.seats[2]
				if seatCarol == nil || !seatCarol.sitOut {
					t.Fatal("expected carol to remain sit-out")
				}
				return
			}
		}
	}
}

func TestRoomSequentialCommandProcessing(t *testing.T) {
	room, err := NewRoom(4, Blinds{Small: 5, Big: 10}, 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go room.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			playerID := "p" + string(rune('A'+i))
			if err := sendCmd(room, SitDown{
				PlayerID: playerID,
				Seat:     i,
				BuyIn:    200 + i*50,
			}); err != nil {
				t.Errorf("sitdown %s: %v", playerID, err)
			}
		}(i)
	}
	wg.Wait()

	if len(room.playerSeat) != 3 {
		t.Fatalf("expected 3 seated players, got %d", len(room.playerSeat))
	}
	for _, seat := range []int{0, 1, 2} {
		if room.seats[seat] == nil {
			t.Fatalf("expected seat %d occupied", seat)
		}
	}
}

func mustSendCmd(t *testing.T, room *Room, cmd Command) {
	t.Helper()
	if err := sendCmd(room, cmd); err != nil {
		t.Fatal(err)
	}
}

func sendCmd(room *Room, cmd Command) error {
	resp := make(chan error, 1)
	switch c := cmd.(type) {
	case SitDown:
		c.Resp = resp
		room.Commands <- c
	case StandUp:
		c.Resp = resp
		room.Commands <- c
	case PlayerAction:
		c.Resp = resp
		room.Commands <- c
	case Disconnect:
		c.Resp = resp
		room.Commands <- c
	case Reconnect:
		c.Resp = resp
		room.Commands <- c
	default:
		return nil
	}

	select {
	case err := <-resp:
		return err
	case <-time.After(2 * time.Second):
		return context.DeadlineExceeded
	}
}
