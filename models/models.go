package models

import (
	"gorm.io/gorm"
)

type Order struct {
	gorm.Model
	UserID           uint           `gorm:"not null"`
	ProductInStockID uint           `gorm:"not null"`
	Product          ProductInStock `gorm:"foreignKey:ProductInStockID"`
	Status           string         `gorm:"type:varchar(20);not null"`
}

// CatalogItem - базовый тип техники (категория + производитель + модель)
type CatalogItem struct {
	gorm.Model
	Category     string `gorm:"not null;size:100;index:idx_category_manufacturer_model"`
	Manufacturer string `gorm:"not null;size:100;index:idx_category_manufacturer_model"`
	DeviceModel  string `gorm:"not null;size:100;index:idx_category_manufacturer_model;column:device_model"`

	//Список конфигураций и дополнительных характеристик
	Stock []ProductInStock `gorm:"foreignKey:CatalogItemID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Specs []ProductSpec    `gorm:"foreignKey:ProductID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	// Description     string `gorm:"type:text"`
}

// ProductInStock - конкретные экземпляры товаров
type ProductInStock struct {
	gorm.Model
	CatalogItemID uint    `gorm:"not null;index"`
	Color         string  `gorm:"not null;size:50;index"`
	Country       string  `gorm:"not null;size:50;column:country_of_origin"`
	Price         float64 `gorm:"not null;check:price > 0"`
	StockQuantity int     `gorm:"not null;default:1"`
	// WarehouseCode string  `gorm:"size:50;index"`
	//   Condition     string  `gorm:"size:20;default:'new'"`

	// Связь обратная
	CatalogItem CatalogItem `gorm:"foreignKey:CatalogItemID"`
}

type SpecType struct {
	gorm.Model
	Name   string `gorm:"not null;uniqueIndex"` // например: "ОЗУ", "Комплектация", "Диагональ"
	Unit   string // например: "ГБ", "дюймов", "шт", "комплект"
	IsList bool   // если true — значение может быть списком (напр., комплектация)
}

type ProductSpec struct {
	gorm.Model
	ProductID  uint `gorm:"not null;index"`
	Product    ProductInStock
	SpecTypeID uint `gorm:"not null;index"`
	SpecType   SpecType
	Value      string `gorm:"not null"` // строковое представление значения
}
