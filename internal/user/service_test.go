package user

import (
	"context"
	"errors"
	"testing"
)

func TestDeleteRejectsOwnMembership(t *testing.T) {
	service := NewService(nil)

	err := service.Delete(context.Background(), 1, 10, 10)

	if !errors.Is(err, ErrSelfDelete) {
		t.Fatalf("expected ErrSelfDelete, got %v", err)
	}
}
