package service

import (
	"context"
	"fmt"

	"github.com/Slimzeo/hev/internal/model"
)

// Store persists the current Environment records used by Service.
type Store interface {
	Create(context.Context, model.Environment) (model.Environment, error)
	Default(context.Context) (model.Environment, error)
	GetByIDOrName(context.Context, string) (model.Environment, error)
	UpdateMany(context.Context, []string, func([]model.Environment) error) ([]model.Environment, error)
}

// IDGenerator supplies stable Environment IDs.
type IDGenerator func() model.EnvironmentID

// Service implements Environment operations independent of any CLI or host.
type Service struct {
	store Store
	newID IDGenerator
}

// New constructs an Environment service.
func New(store Store, newID IDGenerator) *Service {
	return &Service{store: store, newID: newID}
}

// Create creates an empty Environment at revision one.
func (s *Service) Create(ctx context.Context, name string) (model.Environment, error) {
	if err := model.ValidateName(name); err != nil {
		return model.Environment{}, err
	}
	created := model.Environment{
		ID:       s.newID(),
		Name:     name,
		Revision: 1,
		Skills:   []model.EnvironmentSkill{},
	}
	if err := created.Validate(); err != nil {
		return model.Environment{}, err
	}
	return s.store.Create(ctx, created)
}

// Default returns the latest default Environment.
func (s *Service) Default(ctx context.Context) (model.Environment, error) {
	current, err := s.store.Default(ctx)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve default environment: %w", err)
	}
	return current, nil
}

// Resolve returns the latest Environment for an ID or name.
func (s *Service) Resolve(ctx context.Context, identifier string) (model.Environment, error) {
	if identifier == "" {
		return model.Environment{}, model.NewError(model.StatusCodeInvalidArgument, "environment identifier must not be empty")
	}

	current, err := s.store.GetByIDOrName(ctx, identifier)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve environment: %w", err)
	}
	return current, nil
}
