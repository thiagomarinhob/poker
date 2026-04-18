package game

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thiagomarinho/poker-backend/internal/pokerdb"
	"github.com/thiagomarinho/poker-backend/internal/user"
)

// handWriter is o subconjunto de pokerdb.Querier usado pela Room (persistência
// assíncrona de mãos ações, sem tocar em users fora de ApplyChipsDelta).
type handWriter interface {
	InsertHand(ctx context.Context, arg pokerdb.InsertHandParams) (pokerdb.Hand, error)
	CompleteHand(ctx context.Context, arg pokerdb.CompleteHandParams) (pokerdb.Hand, error)
	InsertHandAction(ctx context.Context, arg pokerdb.InsertHandActionParams) (pokerdb.HandAction, error)
}

type userChipsQuerier interface {
	ApplyChipsDelta(ctx context.Context, arg user.ApplyChipsDeltaParams) (user.User, error)
}

// Persistence liga a Room a sqlc: User para buy-in/stand (síncrono) e
// hands/hand_actions (assíncrono via worker). Workers: use 1 para manter
// a ordem das inserções por mão; valores >1 não são atuais.
type Persistence struct {
	RoomID  string
	UserQ   *user.Queries
	HandQ   handWriter
	Workers int
}

// WithPersistence aplica a config; HandQ+RoomID habilita fila assíncrona; UserQ
// habilita ajuste de saldo em SitDown/StandUp.
func WithPersistence(p *Persistence) RoomOption {
	return func(r *Room) error {
		if p == nil {
			return nil
		}
		if p.RoomID != "" {
			r.roomID = p.RoomID
		}
		r.userQ = p.UserQ
		if p.HandQ == nil {
			return nil
		}
		if p.RoomID == "" {
			return fmt.Errorf("persistence: RoomID obrigatório com HandQ")
		}
		r.handQ = p.HandQ
		workers := p.Workers
		if workers < 1 {
			workers = 1
		}
		r.startPersistence(workers)
		return nil
	}
}

type persistJob = func(ctx context.Context) error

func (r *Room) startPersistence(workers int) {
	if r.persistCh != nil {
		return
	}
	if r.handQ == nil {
		return
	}
	if r.roomID == "" {
		return
	}
	if workers < 1 {
		workers = 1
	}
	// Só 1 consumidor: preserva ordem (insert mão -> ações -> complete) no canal.
	if workers > 1 {
		workers = 1
	}
	r.persistCh = make(chan persistJob, 2048)
	r.persistCtx, r.persistCancel = context.WithCancel(context.Background())
	for w := 0; w < workers; w++ {
		r.persistWg.Add(1)
		go func() {
			defer r.persistWg.Done()
			for {
				select {
				case <-r.persistCtx.Done():
					return
				case job, ok := <-r.persistCh:
					if !ok {
						return
					}
					if job == nil {
						continue
					}
					jctx, cancel := context.WithTimeout(r.persistCtx, 15*time.Second)
					_ = job(jctx) // em produção: log + métrica
					cancel()
				}
			}
		}()
	}
}

func (r *Room) enqueuePersistence(job persistJob) {
	if r == nil || r.handQ == nil || r.persistCh == nil {
		return
	}
	select {
	case r.persistCh <- job:
		return
	default:
		// nunca bloquear o ator: descarregar em background
		go func() {
			r.persistCh <- job
		}()
	}
}

// StopPersistence encerra workers (útil em testes com DB real).
func (r *Room) StopPersistence() {
	if r.persistCancel != nil {
		r.persistCancel()
	}
	r.persistWg.Wait()
}

func (r *Room) applyChipsSync(
	userID uuid.UUID,
	delta int64,
	typ string,
	handID *uuid.UUID,
	metadata []byte,
) error {
	if r.userQ == nil {
		return nil
	}
	var hid pgtype.UUID
	if handID != nil {
		hid = pgUUIDFromPtr(handID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err := r.userQ.ApplyChipsDelta(ctx, user.ApplyChipsDeltaParams{
		ID:       userID,
		Amount:   delta,
		Type:     typ,
		HandID:   hid,
		Metadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("chips: %s: %w", typ, err)
	}
	return nil
}

func pgUUIDFromPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	b := *id
	return pgtype.UUID{Valid: true, Bytes: b}
}

func (r *Room) pgUserIDForSeat(seat int) pgtype.UUID {
	if seat < 0 || seat >= r.MaxSeats {
		return pgtype.UUID{Valid: false}
	}
	s := r.seats[seat]
	if s == nil || s.userID == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgUUIDFromPtr(s.userID)
}

func handTotalWagered(h *Hand) int {
	if h == nil {
		return 0
	}
	t := 0
	for _, p := range h.Players {
		t += p.TotalBet
	}
	return t
}

func (r *Room) persistHandAndBlindsStart(hnd *Hand, handID uuid.UUID) {
	if hnd == nil || r.handQ == nil {
		return
	}
	pot := int64(hnd.Pot)
	sbIdx := hnd.SBIndex()
	bbIdx := hnd.BBIndex()
	// Cópias: o job roda fora de ordem e o estado da Room pode ser outra mão.
	roomID := r.roomID
	hc := int32(r.handCounter)
	dealSeat := int32(r.dealerSeat)
	small, big := int64(r.Blinds.Small), int64(r.Blinds.Big)
	street := hnd.Street.String()
	sbAmount := int64(0)
	if sbIdx < len(hnd.Players) {
		sbAmount = int64(hnd.Players[sbIdx].StreetBet)
	}
	bbAmount := int64(0)
	if bbIdx < len(hnd.Players) {
		bbAmount = int64(hnd.Players[bbIdx].StreetBet)
	}
	seatSB := r.handToSeat[sbIdx]
	seatBB := r.handToSeat[bbIdx]
	uidSB := r.pgUserIDForSeat(seatSB)
	uidBB := r.pgUserIDForSeat(seatBB)
	potVal := pot
	potRef := &potVal
	sbi := int32(sbIdx)
	bbi := int32(bbIdx)
	seatSBI := int32(seatSB)
	seatBBI := int32(seatBB)

	r.enqueuePersistence(func(ctx context.Context) error {
		if _, err := r.handQ.InsertHand(ctx, pokerdb.InsertHandParams{
			ID:         handID,
			RoomID:     roomID,
			HandNumber: hc,
			DealerSeat: dealSeat,
			SmallBlind: small,
			BigBlind:   big,
			PotTotal:   potRef,
		}); err != nil {
			return err
		}
		_, err := r.handQ.InsertHandAction(ctx, pokerdb.InsertHandActionParams{
			HandID:          handID,
			ActionSeq:       0,
			TableSeat:       &seatSBI,
			HandPlayerIndex: &sbi,
			UserID:          uidSB,
			ActionType:      PostBlind.String(),
			Amount:          &sbAmount,
			Street:          street,
			IsTimeout:       false,
			Metadata:        nil,
		})
		if err != nil {
			return err
		}
		_, err = r.handQ.InsertHandAction(ctx, pokerdb.InsertHandActionParams{
			HandID:          handID,
			ActionSeq:       1,
			TableSeat:       &seatBBI,
			HandPlayerIndex: &bbi,
			UserID:          uidBB,
			ActionType:      PostBlind.String(),
			Amount:          &bbAmount,
			Street:          street,
			IsTimeout:       false,
			Metadata:        nil,
		})
		if err != nil {
			return err
		}
		return nil
	})
	// ações a partir de agora usam 2+ (0 e 1 = blinds) — fora do job assíncrono
	// (define sequência de action_seq alinhada ao ator, não à ordem de flush no banco)
	r.nextActionSeq = 2
}

func (r *Room) persistHandAction(
	handID uuid.UUID,
	seat int,
	hi int,
	typ ActionType,
	amount *int64,
	street string,
	isTimeout bool,
) {
	if r.handQ == nil {
		return
	}
	s := r.nextActionSeq
	r.nextActionSeq++
	seatI := int32(seat)
	hip := int32(hi)
	uid := r.pgUserIDForSeat(seat)
	snapHand := handID
	snap := s
	typS := typ.String()
	streetC := street
	amtC := amount
	tmo := isTimeout
	r.enqueuePersistence(func(ctx context.Context) error {
		_, err := r.handQ.InsertHandAction(ctx, pokerdb.InsertHandActionParams{
			HandID:          snapHand,
			ActionSeq:       snap,
			TableSeat:       &seatI,
			HandPlayerIndex: &hip,
			UserID:          uid,
			ActionType:      typS,
			Amount:          amtC,
			Street:          streetC,
			IsTimeout:       tmo,
			Metadata:        nil,
		})
		return err
	})
}

func (r *Room) persistHandComplete(hnd *Hand, handID uuid.UUID) {
	if hnd == nil || r.handQ == nil {
		return
	}
	totalWagered := int64(handTotalWagered(hnd))
	pt := &totalWagered
	r.enqueuePersistence(func(ctx context.Context) error {
		_, err := r.handQ.CompleteHand(ctx, pokerdb.CompleteHandParams{
			ID:       handID,
			PotTotal: pt,
		})
		return err
	})
}
