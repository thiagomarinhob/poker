package table

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

func validationMsg(err error) string {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		f := ve[0]
		return f.Field() + " invalid (" + f.Tag() + ")"
	}
	return "validation failed"
}

var createTableValidator = validator.New()

// CreateTableBody é o corpo JSON compartilhado entre criação e atualização de mesa (admin).
type CreateTableBody struct {
	Name               string `json:"name"                 validate:"required,min=1,max=120"`
	MaxSeats           int    `json:"max_seats"            validate:"required,min=2,max=10"`
	SmallBlind         int    `json:"small_blind"          validate:"required,min=1"`
	BigBlind           int    `json:"big_blind"            validate:"required,min=1"`
	TurnTimeoutSeconds int    `json:"turn_timeout_seconds" validate:"required,min=5,max=600"`
}

func (b CreateTableBody) TurnDuration() time.Duration {
	return time.Duration(b.TurnTimeoutSeconds) * time.Second
}

func (b CreateTableBody) Validate() error {
	if err := createTableValidator.Struct(b); err != nil {
		return errors.New(validationMsg(err))
	}
	if b.SmallBlind > b.BigBlind {
		return fmt.Errorf("small_blind must be <= big_blind")
	}
	return nil
}
