package application

import (
	"context"

	"github.com/Slimzeo/hev/internal/domain"
)

// EnvironmentStore owns the current persisted Environment records.
type EnvironmentStore interface {
	Create(context.Context, domain.Environment) (domain.Environment, error)
	GetManyByIDOrName(context.Context, []string) ([]domain.Environment, error)
	UpdateMany(
		context.Context,
		[]string,
		func([]domain.Environment) error,
	) ([]domain.Environment, error)
}
