package db

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"tg_bot/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type (
	CallbackQuery = models.CallbackQuery
	CallBackData  = models.CallBackData
	Message       = models.Message
	Update        = models.Update
)

func Connect() (*gorm.DB, error) {
	db_host := os.Getenv("DB_HOST")
	db_user := os.Getenv("DB_USER")
	db_password := os.Getenv("DB_PASSWORD")
	db_name := os.Getenv("DB_NAME")
	db_port := os.Getenv("DB_PORT")

	dsn := "host=" + db_host + " user=" + db_user + " password=" + db_password + " dbname=" + db_name + " port=" + db_port + " sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Ошибка подключения к базе:", err)
	}
	return db, err
}

// Обработка сообщения редактирования списка конфигураций
func RedactProductsInStock(DB *gorm.DB, msg *Message) (int, error) {
	lines := strings.Split(msg.Text, "\n")
	startParsing := false
	updated := 0

	for _, line := range lines {
		// Начать парсинг после заголовка таблицы
		if strings.HasPrefix(line, "ID |") {
			startParsing = true
			continue
		}
		if !startParsing || strings.HasPrefix(line, "---") || strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		idStr := strings.TrimSpace(parts[0])
		priceStr := strings.TrimSpace(parts[3])
		qtyStr := strings.TrimSpace(parts[4])

		id, err := strconv.Atoi(idStr)
		if err != nil {
			return 0, fmt.Errorf("не удалось распарсить ID: %v", err)
		}
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return 0, fmt.Errorf("не удалось распарсить цену для ID %d: %v", id, err)
		}
		qty, err := strconv.Atoi(qtyStr)
		if err != nil {
			return 0, fmt.Errorf("не удалось распарсить количество для ID %d: %v", id, err)
		}

		var product models.ProductInStock

		if err := DB.First(&product, id).Error; err != nil {
			return 0, fmt.Errorf("не удалось найти Товар в каталоге с ID %d: %v", id, err)
		}

		// Проверяем, изменились ли данные
		if product.Price != price || product.StockQuantity != qty {
			product.Price = price
			product.StockQuantity = qty
			if err := DB.Save(product).Error; err != nil {
				return updated, fmt.Errorf("ошибка при сохранении Stock ID %d: %v", id, err)
			}
			updated++
		}
	}

	return updated, nil
}

// Выдает все модели для заданной категории и производиеля
func Get_device_models(DB *gorm.DB, category, manufacturer string) ([]string, error) {
	var res []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("device_model").
		Where("category = ? AND manufacturer = ?", category, manufacturer).
		Order("device_model ASC").
		Pluck("device_model", &res).
		Error
	return res, err
}

func SeedTestData(DB *gorm.DB) error {
	// Фиксированные тестовые данные
	testCatalogItems := []models.CatalogItem{
		// Ноутбуки
		{Category: "Ноутбуки", Manufacturer: "Apple", DeviceModel: "MacBook Pro 16"},
		{Category: "Ноутбуки", Manufacturer: "Apple", DeviceModel: "MacBook Air M1"},
		{Category: "Ноутбуки", Manufacturer: "Lenovo", DeviceModel: "ThinkPad X1 Carbon"},
		{Category: "Ноутбуки", Manufacturer: "Asus", DeviceModel: "ZenBook Pro Duo"},
		{Category: "Ноутбуки", Manufacturer: "Dell", DeviceModel: "XPS 15"},

		// Смартфоны
		{Category: "Смартфоны", Manufacturer: "Apple", DeviceModel: "iPhone 15 Pro"},
		{Category: "Смартфоны", Manufacturer: "Samsung", DeviceModel: "Galaxy S23 Ultra"},
		{Category: "Смартфоны", Manufacturer: "Xiaomi", DeviceModel: "Redmi Note 12"},
		{Category: "Смартфоны", Manufacturer: "Google", DeviceModel: "Pixel 7 Pro"},
		{Category: "Смартфоны", Manufacturer: "OnePlus", DeviceModel: "11 Pro"},

		// Планшеты
		{Category: "Планшеты", Manufacturer: "Apple", DeviceModel: "iPad Pro 12.9"},
		{Category: "Планшеты", Manufacturer: "Samsung", DeviceModel: "Galaxy Tab S9"},
		{Category: "Планшеты", Manufacturer: "Huawei", DeviceModel: "MatePad Pro"},
		{Category: "Планшеты", Manufacturer: "Lenovo", DeviceModel: "Tab P12"},
		{Category: "Планшеты", Manufacturer: "Xiaomi", DeviceModel: "Pad 6"},
	}

	// Создаем CatalogItems
	for i := range testCatalogItems {
		if err := DB.Create(&testCatalogItems[i]).Error; err != nil {
			return fmt.Errorf("failed to create catalog item: %v", err)
		}

		// Создаем товары для каждого CatalogItem
		products := []models.ProductInStock{
			{
				CatalogItemID: testCatalogItems[i].ID,
				Color:         "Black",
				Country:       "USA",
				Price:         999.99,
				StockQuantity: 5,
			},
			{
				CatalogItemID: testCatalogItems[i].ID,
				Color:         "White",
				Country:       "China",
				Price:         899.99,
				StockQuantity: 3,
			},
		}

		// Для некоторых товаров добавляем третий вариант
		if i%2 == 0 {
			products = append(products, models.ProductInStock{
				CatalogItemID: testCatalogItems[i].ID,
				Color:         "Silver",
				Country:       "Vietnam",
				Price:         1099.99,
				StockQuantity: 2,
			})
		}

		for j := range products {
			if err := DB.Create(&products[j]).Error; err != nil {
				return fmt.Errorf("failed to create product in stock: %v", err)
			}
		}
	}

	return nil
}

// Уникальные категории из каталога
func Get_categories(DB *gorm.DB) ([]string, error) {
	var categories []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("category").
		Order("category ASC").
		Pluck("category", &categories).
		Error
	return categories, err
}

// Выдает всех производителей для заданной категории
func Get_manufacturers(DB *gorm.DB, category string) ([]string, error) {
	var manufacturers []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("manufacturer").
		Where("category = ?", category).
		Order("manufacturer ASC").
		Pluck("manufacturer", &manufacturers).
		Error
	return manufacturers, err
}

// (АДМИН) добавляет новую конфигурацию для заданной модели
func AddProductConfiguration(
	DB *gorm.DB,
	category string,
	manufacturer string,
	deviceModel string,
	config *models.ProductInStock,
) error {

	var item models.CatalogItem

	// Поиск CatalogItem
	err := DB.Where("category = ? AND manufacturer = ? AND device_model = ?", category, manufacturer, deviceModel).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("catalog item not found")
		}
		return err
	}
	config.CatalogItemID = item.ID

	return DB.Create(&config).Error
}

// (ПОЛЬЗОВАТЕЛЬ) Добавление заказа в БД
func MakeOrder(DB *gorm.DB, userID int64, productID uint) error {
	// Создание нового заказа
	order := models.Order{
		UserID:           uint(userID),
		ProductInStockID: productID,
		Status:           "created",
	}

	// Добавление заказа в базу данных
	if err := DB.Create(&order).Error; err != nil {
		log.Printf("Ошибка при создании заказа: %v", err)
		return fmt.Errorf("ошибка при создании заказа на отовар %d", productID)
	}
	return nil
}

func Get_configurations(DB *gorm.DB, category, manufacturer, model string) ([]models.ProductInStock, error) {
	var catalogItem models.CatalogItem

	err := DB.Where("category = ? AND manufacturer = ? AND device_model = ?",
		category, manufacturer, model).
		Preload("Stock").
		First(&catalogItem).Error
	if err != nil {
		return nil, fmt.Errorf("не удалось найти товар: %v", err)
	}
	return catalogItem.Stock, nil
}

// (АДМИН) Удаляет конфигурацию по ID
func RemoveProduct(DB *gorm.DB, msg *Message) error {
	str := strings.TrimSpace(msg.Text)
	id, err := strconv.Atoi(str)
	if err != nil {
		//utils.SendMessage(msg.Chat.ID, )
		return fmt.Errorf("некорректный ID")
	}

	var product models.ProductInStock
	if err := DB.First(&product, id).Error; err != nil {
		return fmt.Errorf("товар с ID %d не найден", id)
	}

	if err := DB.Delete(&product).Error; err != nil {
		//utils.SendMessage(msg.Chat.ID, fmt.Sprintf("Ошибка при удалении товара с ID %d: %v", id, err))
		return fmt.Errorf("ошибка при удалении товара с ID %d: %v", id, err)
	}
	// fmt.Errorf("товар с ID %d успешно удалён", id)
	return nil
}
