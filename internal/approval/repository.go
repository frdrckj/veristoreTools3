package approval

import "gorm.io/gorm"

// Repository provides database access for CSI approval requests.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new approval repository.
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new CSI request.
func (r *Repository) Create(req *CsiRequest) error {
	return r.db.Create(req).Error
}

// FindByID retrieves a CSI request by ID.
func (r *Repository) FindByID(id int) (*CsiRequest, error) {
	var req CsiRequest
	if err := r.db.Where("req_id = ?", id).First(&req).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

// FindPending returns all pending CSI requests, newest first.
func (r *Repository) FindPending() ([]CsiRequest, error) {
	var requests []CsiRequest
	err := r.db.Where("req_status = ?", "PENDING").Order("req_id DESC").Find(&requests).Error
	return requests, err
}

// FindAll returns all CSI requests, newest first.
func (r *Repository) FindAll() ([]CsiRequest, error) {
	var requests []CsiRequest
	err := r.db.Order("req_id DESC").Find(&requests).Error
	return requests, err
}

// ApprovalFilter holds filter parameters for the approval list.
type ApprovalFilter struct {
	CSI       string
	Source    string
	CreatedBy string
	Status    string
	DateFrom  string
	DateTo    string
}

// FindPaginated returns paginated CSI requests with total count and filters.
func (r *Repository) FindPaginated(page, perPage int, filter ApprovalFilter) ([]CsiRequest, int64, error) {
	tx := r.db.Model(&CsiRequest{})

	if filter.CSI != "" {
		tx = tx.Where("req_device_id LIKE ?", "%"+filter.CSI+"%")
	}
	if filter.Source != "" {
		tx = tx.Where("req_source = ?", filter.Source)
	}
	if filter.CreatedBy != "" {
		tx = tx.Where("created_by LIKE ?", "%"+filter.CreatedBy+"%")
	}
	if filter.Status != "" {
		tx = tx.Where("req_status = ?", filter.Status)
	}
	if filter.DateFrom != "" {
		tx = tx.Where("DATE(created_dt) >= ?", filter.DateFrom)
	}
	if filter.DateTo != "" {
		tx = tx.Where("DATE(created_dt) <= ?", filter.DateTo)
	}

	var total int64
	tx.Count(&total)

	var requests []CsiRequest
	offset := (page - 1) * perPage
	err := tx.Order("req_id DESC").Limit(perPage).Offset(offset).Find(&requests).Error
	return requests, total, err
}

// GetDistinctCreators returns distinct created_by values for the filter dropdown.
func (r *Repository) GetDistinctCreators() []string {
	var creators []string
	r.db.Model(&CsiRequest{}).Distinct("created_by").Where("created_by != ''").Pluck("created_by", &creators)
	return creators
}

// FindByIDs returns requests matching the given IDs.
func (r *Repository) FindByIDs(ids []int) ([]CsiRequest, error) {
	var requests []CsiRequest
	err := r.db.Where("req_id IN ?", ids).Find(&requests).Error
	return requests, err
}

// UpdateStatus updates the status and approval info.
func (r *Repository) UpdateStatus(req *CsiRequest) error {
	return r.db.Model(&CsiRequest{}).Where("req_id = ?", req.ReqID).Updates(map[string]interface{}{
		"req_status":  req.Status,
		"approved_by": req.ApprovedBy,
		"approved_dt": req.ApprovedDt,
	}).Error
}

// Delete removes a CSI request by ID.
func (r *Repository) Delete(id int) error {
	return r.db.Delete(&CsiRequest{}, "req_id = ?", id).Error
}

// PendingCount returns the number of pending requests.
func (r *Repository) PendingCount() int64 {
	var count int64
	r.db.Model(&CsiRequest{}).Where("req_status = ?", "PENDING").Count(&count)
	return count
}
