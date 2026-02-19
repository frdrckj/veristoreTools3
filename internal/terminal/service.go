package terminal

import (
	"fmt"

	"gorm.io/gorm"
)

// Service provides business logic for terminal operations.
type Service struct {
	repo *Repository
	db   *gorm.DB
}

// NewService creates a new terminal service.
func NewService(repo *Repository, db *gorm.DB) *Service {
	return &Service{
		repo: repo,
		db:   db,
	}
}

// GetTerminals returns a paginated and optionally filtered list of terminals.
func (s *Service) GetTerminals(page, perPage int, search string) ([]Terminal, int64, error) {
	terminals, pagination, err := s.repo.Search(search, page, perPage)
	if err != nil {
		return nil, 0, err
	}
	return terminals, pagination.Total, nil
}

// GetTerminalByID retrieves a terminal by its primary key.
func (s *Service) GetTerminalByID(id int) (*Terminal, error) {
	return s.repo.FindByID(id)
}

// GetTerminalByCSI retrieves the first terminal matching the given serial number (CSI).
func (s *Service) GetTerminalByCSI(csi string) (*Terminal, error) {
	terminals, err := s.repo.FindByCSI(csi)
	if err != nil {
		return nil, err
	}
	if len(terminals) == 0 {
		return nil, fmt.Errorf("terminal with CSI %q not found", csi)
	}
	return &terminals[0], nil
}

// CreateTerminal inserts a new terminal record.
func (s *Service) CreateTerminal(t *Terminal) error {
	return s.repo.Create(t)
}

// UpdateTerminal saves changes to an existing terminal record.
func (s *Service) UpdateTerminal(t *Terminal) error {
	return s.repo.Update(t)
}

// DeleteTerminal removes a terminal by ID.
func (s *Service) DeleteTerminal(id int) error {
	return s.repo.Delete(id)
}

// GetTerminalParameters retrieves all parameters for a given terminal.
func (s *Service) GetTerminalParameters(terminalID int) ([]TerminalParameter, error) {
	return s.repo.FindParametersByTermID(terminalID)
}
