package user

import (
	"github.com/verifone/veristoretools3/internal/shared"
	"gorm.io/gorm"
)

// Repository provides data access for the User model.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new user repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindByID retrieves a user by their primary key.
func (r *Repository) FindByID(id int) (*User, error) {
	var u User
	if err := r.db.First(&u, "user_id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByUsername retrieves a user by username.
func (r *Repository) FindByUsername(username string) (*User, error) {
	var u User
	if err := r.db.Where("user_name = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// Search returns a paginated list of users matching the given query.
// The query string is matched against user_fullname and user_name.
func (r *Repository) Search(query string, page, perPage int) ([]User, shared.Pagination, error) {
	var users []User
	var total int64

	tx := r.db.Model(&User{})
	if query != "" {
		like := "%" + query + "%"
		tx = tx.Where("user_fullname LIKE ? OR user_name LIKE ?", like, like)
	}

	if err := tx.Count(&total).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	p := shared.NewPagination(page, perPage, total)
	if err := tx.Offset(p.Offset()).Limit(p.PerPage).Order("user_id DESC").Find(&users).Error; err != nil {
		return nil, shared.Pagination{}, err
	}

	return users, p, nil
}

// All returns every user in the table.
func (r *Repository) All() ([]User, error) {
	var users []User
	if err := r.db.Order("user_id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Create inserts a new user record.
func (r *Repository) Create(u *User) error {
	return r.db.Create(u).Error
}

// Update saves changes to an existing user record.
func (r *Repository) Update(u *User) error {
	return r.db.Save(u).Error
}

// Delete removes a user by ID.
func (r *Repository) Delete(id int) error {
	return r.db.Delete(&User{}, "user_id = ?", id).Error
}

// UpdateStatus sets the status field for the given user.
func (r *Repository) UpdateStatus(id int, status int) error {
	return r.db.Model(&User{}).Where("user_id = ?", id).Update("status", status).Error
}
