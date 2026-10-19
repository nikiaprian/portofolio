package entity

import "time"

const (
	PortfolioPending      = "pending"
	PortfolioActive       = "active"
	PortfolioSuspended    = "suspended"
	PortfolioOrderPending = "pending"
	PortfolioOrderPaid    = "paid"
	PortfolioOrderDeny    = "deny"
	PortfolioOrderCancel  = "cancel"
	PortfolioOrderExpire  = "expire"
)

// PortfolioSkill is one skill chip on the public portfolio.
type PortfolioSkill struct {
	Name string `json:"name"`
}

// PortfolioProject is one project card on the public portfolio.
type PortfolioProject struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	URL     string `json:"url"`
}

// PortfolioLinks holds optional profile links.
type PortfolioLinks struct {
	Website   string `json:"website"`
	Instagram string `json:"instagram"`
	LinkedIn  string `json:"linkedin"`
	GitHub    string `json:"github"`
}

// Portfolio is the buyer site served on {subdomain}.{root}.
type Portfolio struct {
	ID          uint               `gorm:"primaryKey" json:"id"`
	Subdomain   string             `gorm:"size:30;uniqueIndex;not null" json:"subdomain"`
	Status      string             `gorm:"size:20;index;not null;default:pending" json:"status"`
	DisplayName string             `gorm:"size:100;not null;default:''" json:"display_name"`
	Headline    string             `gorm:"size:160;not null;default:''" json:"headline"`
	Bio         string             `gorm:"type:text" json:"bio"`
	PhotoURL    string             `gorm:"size:500;not null;default:''" json:"photo_url"`
	Email       string             `gorm:"size:150;not null;default:''" json:"email"`
	WhatsApp    string             `gorm:"size:20;not null;default:''" json:"whatsapp"`
	Links       PortfolioLinks     `gorm:"serializer:json" json:"links"`
	Skills      []PortfolioSkill   `gorm:"serializer:json" json:"skills"`
	Projects    []PortfolioProject `gorm:"serializer:json" json:"projects"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// PortfolioOrder is a Midtrans checkout for one portfolio subdomain.
type PortfolioOrder struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	PortfolioID       uint      `gorm:"index;not null" json:"portfolio_id"`
	PublicRef         string    `gorm:"size:64;uniqueIndex;not null" json:"public_ref"`
	SetupToken        string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	MidtransOrderID   string    `gorm:"size:64;uniqueIndex;not null" json:"midtrans_order_id"`
	SnapToken         string    `gorm:"type:text" json:"-"`
	BuyerName         string    `gorm:"size:100;not null" json:"buyer_name"`
	Email             string    `gorm:"size:150;index;not null" json:"email"`
	WhatsApp          string    `gorm:"size:20;not null;default:''" json:"whatsapp"`
	Subdomain         string    `gorm:"size:30;index;not null" json:"subdomain"`
	Amount            int64     `gorm:"not null" json:"amount"`
	Status            string    `gorm:"size:20;index;not null;default:pending" json:"status"`
	TransactionStatus string    `gorm:"size:30;not null;default:''" json:"transaction_status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PortfolioAccount is the buyer login. It is not a blog user.
type PortfolioAccount struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PortfolioID uint      `gorm:"uniqueIndex;not null" json:"portfolio_id"`
	Email       string    `gorm:"size:150;uniqueIndex;not null" json:"email"`
	Password    string    `gorm:"size:255;not null;default:''" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
