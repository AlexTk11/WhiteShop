package messages

import (
	"fmt"
	"strconv"
	"strings"
	"tg_bot/models"

	"gorm.io/gorm"
)

// (АДМИН) Создает образец сообщения для редактирования конфигураций товара
func MakeMessage_GetConfigurationsForEdit(stock []models.ProductInStock) string {

	var message strings.Builder
	message.WriteString("Скопируйте данное сообщение и отправьте с внесенными изменениями.\n")
	message.WriteString("(редактировать можно цену и количество)\n")
	// Добавляем список конфигураций
	if len(stock) == 0 {
		message.WriteString("Нет доступных конфигураций")
	} else {
		message.WriteString("Доступные конфигурации:\n\n")
		message.WriteString("ID | Цвет | Страна | Цена | Кол-во\n")
		message.WriteString("----------------------------------\n")

		for _, stock := range stock {
			message.WriteString(fmt.Sprintf("%d | %s | %s | %.2f | %d\n",
				stock.ID,
				stock.Color,
				stock.Country,
				stock.Price,
				stock.StockQuantity))
		}
	}
	return message.String()
}

// (АДМИН) Парсинг измененных конфигураций товара из сообщения
func ParseNewConfig(msg string) (product models.ProductInStock, model, category, manufacturer string, err error) {
	lines := strings.Split(strings.TrimSpace(msg), "\n")

	product = models.ProductInStock{}

	if len(lines) < 2 {
		err = fmt.Errorf("недостаточно строк для парсинга: ожидалось как минимум 2, получено %d", len(lines))
		return
	}

	// Первая строка — category | manufacturer | model
	header := strings.Split(lines[0], "|")
	if len(header) != 3 {
		err = fmt.Errorf("ошибка парсинга заголовка: ожидалось 3 части, разделённые '|', получено %d", len(header))
		return
	}

	category = strings.TrimSpace(header[0])
	manufacturer = strings.TrimSpace(header[1])
	model = strings.TrimSpace(header[2])

	// Вторая строка — ключи и значения
	pairs := strings.Split(lines[1], ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		kv := strings.SplitN(pair, ":", 2)
		if len(kv) != 2 {
			continue // Пропускаем некорректные пары
		}

		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])

		switch key {
		case "цвет":
			product.Color = value
		case "страна":
			product.Country = value
		case "цена":
			price, parseErr := strconv.ParseFloat(value, 64)
			if parseErr != nil {
				err = fmt.Errorf("не удалось распарсить цену: %v", parseErr)
				return
			}
			product.Price = price
		case "количество":
			quantity, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				err = fmt.Errorf("не удалось распарсить количество: %v", parseErr)
				return
			}
			product.StockQuantity = quantity
		}
	}

	return
}

// Создает список всех конфигураций для заданной модели
func MakeMessagePriceList(DB *gorm.DB, category, manufacturer, model string) (string, error) {

	var catalogItem models.CatalogItem
	var message strings.Builder

	err := DB.Where("category = ? AND manufacturer = ? AND device_model = ?",
		category, manufacturer, model).
		Preload("Stock").
		First(&catalogItem).Error
	if err != nil {
		return "", fmt.Errorf("не удалось найти товар: %v", err)
	}

	if len(catalogItem.Stock) == 0 {
		message.WriteString("Нет доступных конфигураций")
	} else {
		message.WriteString(fmt.Sprintf("Доступные конфигурации %s:\n\n", catalogItem.DeviceModel))
		message.WriteString("ID | Цвет | Страна | Цена | Кол-во\n")
		message.WriteString("----------------------------------\n")

		for _, stock := range catalogItem.Stock {
			message.WriteString(fmt.Sprintf("%d | %s | %s | %.2f | %d\n",
				stock.ID,
				stock.Color,
				stock.Country,
				stock.Price,
				stock.StockQuantity))
		}
	}
	return message.String(), nil
}

// Создает шаблон для соощения добавления новой конфигурации
func MakeMessage_GetNewConfig(data models.CallBackData, UserId int64) string {
	message := fmt.Sprintf(
		"Добавление новой конфигурации\n"+
			"Введите данные в следующем формате:\n"+
			"```\n%s | %s | %s\nЦвет: Color, Страна: Country, Цена: Price, Количество: Quantity\n```",
		data.Category,
		data.Manufacturer,
		data.Model)

	return message
}
