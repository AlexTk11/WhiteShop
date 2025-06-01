package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"tg_bot/auth"
	"tg_bot/internal/bot_commands"

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

var DB *gorm.DB

var adminStates map[int64]int

func code_request(data CallBackData) string { // кодирует запрос в json строку
	return strings.Join([]string{data.Command, data.Category, data.Manufacturer, data.Model, data.ModelID}, "|")
}
func decode_request(encoded string) CallBackData {
	parts := strings.Split(encoded, "|")

	data := CallBackData{}

	if len(parts) > 0 {
		data.Command = parts[0]
	}
	if len(parts) > 1 {
		data.Category = parts[1]
	}
	if len(parts) > 2 {
		data.Manufacturer = parts[2]
	}
	if len(parts) > 3 {
		data.Model = parts[3]
	}
	if len(parts) > 4 {
		data.ModelID = parts[4]
	}

	return data
}

func setWebhook(webhookURL string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", botToken)

	data := map[string]string{"url": webhookURL}
	json_data, _ := json.Marshal(data)

	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(json_data))
	if err != nil {
		log.Fatal("Ошибка при установке webhook:", err)
	}
	defer resp.Body.Close()

	log.Println("Webhook установлен:", resp.Status)
}

func ShowStartMenu_admin(chatID int64) {

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "Редактирвать каталог",
				"callback_data": code_request(CallBackData{Command: bot_commands.Start_redact})}},
			//{{"text": "Добавить в каталог", "callback_data": code_request(CallBackData{Command: "add_catalog_type"})}},
		},
		"resize_keyboard": true,
	}
	jsonData, _ := json.Marshal(map[string]interface{}{
		"chat_id":      chatID,
		"text":         "Выберите действие:",
		"reply_markup": keyboard,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Println("Ошибка отправки меню:", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("Telegram API Response: %s", string(respBody))

}

func ShowStartMenu_user(chatID int64) {
	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{
			{{"text": "Каталог", "callback_data": code_request(CallBackData{Command: bot_commands.Catalog_start})}},
			{{"text": "Вопрос по заказу", "callback_data": code_request(CallBackData{Command: bot_commands.CheckOrder})}},
		},
		"resize_keyboard": true,
	}
	//fmt.Println(code_request(CallBackData{Command: "add_catalog_start"}))

	jsonData, _ := json.Marshal(map[string]interface{}{
		"chat_id":      chatID,
		"text":         "Меню:",
		"reply_markup": keyboard,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Println("Ошибка отправки меню:", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("Telegram API Response: %s", string(respBody))
}

func user_callback_handler(query *CallbackQuery, botToken string) {
	data := decode_request(query.Data)

	fmt.Println(data.Command)
	switch data.Command {

	case bot_commands.Start:
		{
			ShowStartMenu_user(query.Message.Chat.ID)
			return
		}

	case bot_commands.Catalog_start: //Редактирование каталога(выбор девайса) -> меню с выбором категории
		{
			categories, err := Get_categories()
			if err != nil {
				fmt.Println("Ошибка чтения category в БД")
			}
			ChooseDevice(categories, bot_commands.Choose_category, data, uint64(query.Message.Chat.ID))
			return
		}

	case bot_commands.Choose_category:
		{ //Выбор категории -> выбор производителя
			if data.Category == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			manufacturers, err := Get_manufacturers(data.Category)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			ChooseDevice(manufacturers, bot_commands.Choose_manufacturer, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_manufacturer:
		{ //Выбор производителя -> выбор модели

			if data.Category == "" || data.Manufacturer == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			device_models, err := Get_device_models(data.Category, data.Manufacturer)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			ChooseDevice(device_models, bot_commands.Choose_model, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_model:
		{ //выбор модели -> список всех конфигураций для выбранной модели
			//sendPrices(que)
			msg, err := MakeMessagePriceList(data.Category, data.Manufacturer, data.Model)
			if err != nil {
				fmt.Println("Ошибка извлечения записей из Products")
			} else {
				sendMessage(query.Message.Chat.ID, msg)
			}

			GetUserAction(query.Message.Chat.ID, data, botToken)
			return
		}
	case bot_commands.MakeOrder: //Выбрал конфигурацию -> оформить заказ
		{
			var productID int
			var err error
			if productID, err = strconv.Atoi(data.ModelID); err != nil {
				fmt.Println("Некорректные данные в data.ModelID")
				sendMessage(query.Message.Chat.ID, "Ошибка при создании заказа. Попробуйте позже")
				return
			}

			if err = makeOrder(query.Message.Chat.ID, uint(productID)); err != nil {
				fmt.Printf("Ошибка создания заказа %d", productID)
				sendMessage(query.Message.Chat.ID, err.Error())
				return
			}
			// Рассылка админам нового заказа
			orderMessage := fmt.Sprintf("Новый заказ от пользователя @%s\n%s %s (ID: %d).", query.From.Username, data.Model, data.Manufacturer, productID)

			for admin := range adminStates {
				sendMessage(admin, orderMessage)
			}

			sendMessage(query.Message.Chat.ID, "Заказ успешно оформлен!\n Скоро с вами свяжется менеджер для уточнения деталей.")
			return
		}

	default:
		sendMessage(query.Message.Chat.ID, "Unknown user command")

	}

	fmt.Println(data.Command)
}

// (ПОЛЬЗОВАТЕЛЬ) Добавление заказа в БД
func makeOrder(userID int64, productID uint) error {
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

// ChooseDevice - формирует и отправляет пользователю меню для определенного этапа выбора в каталоге
// category_list - список категорий/производителей/моделей, составляющий кнопки
// command - текущая команда (Choose_category_command / Choose_manufacturer_command / Choose_model_command)
// data - callbackData предыдущего запроса (выбранные на текущий момент характеристики)
// id - UserID получателя
func ChooseDevice(category_list []string, command string,
	data CallBackData, id uint64) {

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{},
		"resize_keyboard": true,
	}
	new_data := data

	for _, ctg := range category_list { //Выводим кнопки с категориями
		new_data.Command = command

		switch command {
		case bot_commands.Choose_category:
			new_data.Category = ctg

		case bot_commands.Choose_manufacturer:
			new_data.Manufacturer = ctg

		case bot_commands.Choose_model:
			new_data.Model = ctg
		default:
			fmt.Println("Unknown command in ChooseDevice: ", command)
			return
		}

		newButton := map[string]string{
			"text":          ctg,
			"callback_data": code_request(new_data),
		}

		newRow := []map[string]string{newButton}

		rows := keyboard["inline_keyboard"].([][]map[string]string)
		keyboard["inline_keyboard"] = append(rows, newRow)
	}

	back_data := data // Добавляем кнопку "Назад"
	switch command {
	case bot_commands.Choose_category: //Категория -> Стартовое меню
		back_data = CallBackData{Command: bot_commands.Start}
	case bot_commands.Choose_manufacturer: // Производитель -> категория
		back_data.Command = bot_commands.Start_redact
	case bot_commands.Choose_model: // Модель -> производитель
		back_data.Command = bot_commands.Choose_category
	default:
		fmt.Println("Unknown command in ChooseDevice: ", command)
		return
	}

	backButton := map[string]string{
		"text":          "Назад",
		"callback_data": code_request(back_data),
	}
	backRow := []map[string]string{backButton}
	rows := keyboard["inline_keyboard"].([][]map[string]string)
	keyboard["inline_keyboard"] = append(rows, backRow)

	jsonData, _ := json.Marshal(map[string]interface{}{
		"chat_id":      id,
		"text":         "Выберите категорию:",
		"reply_markup": keyboard,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Println("Ошибка отправки command add configuration:", err)
		return
	}
	defer resp.Body.Close()
}

// (ПОЛЬЗОВАТЕЛЬ) Отображение клавиатуры с конфигурациями для заданной модели (CallbackData)
func GetUserAction(userID int64, data CallBackData, botToken string) error {

	var catalogItem models.CatalogItem

	err := DB.Where("category = ? AND manufacturer = ? AND device_model = ?",
		data.Category, data.Manufacturer, data.Model).
		Preload("Stock").
		First(&catalogItem).Error
	if err != nil {
		return fmt.Errorf("не удалось найти товар: %v", err)
	}

	if len(catalogItem.Stock) == 0 {
		return fmt.Errorf("нет доступных конфигураций")
	}

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{},
		"resize_keyboard": true,
	}

	for _, device := range catalogItem.Stock {
		buttonText := fmt.Sprintf("%s %s %s --- %.2f ₽", data.Model, device.Color, device.Country, device.Price)
		but_data := data
		but_data.Command = bot_commands.MakeOrder
		but_data.ModelID = strconv.Itoa(int(device.ID))

		button := map[string]string{
			"text":          buttonText,
			"callback_data": code_request(but_data),
		}
		newRow := []map[string]string{button}

		rows := keyboard["inline_keyboard"].([][]map[string]string)
		keyboard["inline_keyboard"] = append(rows, newRow)
	}

	button := map[string]string{
		"text":          "Главное меню",
		"callback_data": code_request(models.CallBackData{Command: bot_commands.Start}),
	}
	newRow := []map[string]string{button}

	rows := keyboard["inline_keyboard"].([][]map[string]string)
	keyboard["inline_keyboard"] = append(rows, newRow)

	jsonData, _ := json.Marshal(map[string]interface{}{
		"chat_id":      userID,
		"text":         "Выберите конфигурацию:",
		"reply_markup": keyboard,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Println("Ошибка отправки get user action", err)
		return err
	}
	defer resp.Body.Close()

	return nil
}

// (АДМИН) Создает образец
func MakeMessage_GetConfigurationsForEdit(category, manufacturer, model string) (string, error) {
	var catalogItem models.CatalogItem
	var message strings.Builder

	err := DB.Where("category = ? AND manufacturer = ? AND device_model = ?",
		category, manufacturer, model).
		Preload("Stock").
		First(&catalogItem).Error
	if err != nil {
		return "", fmt.Errorf("не удалось найти товар: %v", err)
	}

	message.WriteString("Скопируйте данное сообщение и отправьте с внесенными изменениями.\n")
	message.WriteString("(редактировать можно цену и количество)\n")
	// Добавляем список конфигураций
	if len(catalogItem.Stock) == 0 {
		message.WriteString("Нет доступных конфигураций")
	} else {
		message.WriteString("Доступные конфигурации:\n\n")
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

// (АДМИН) добавляет новую конфигурацию для заданной модели
func AddProductConfiguration(
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

// (АДМИН) Парсинг конфигурации из сообщения
func parseNewConfig(msg string) (product models.ProductInStock, model, category, manufacturer string, err error) {
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
func MakeMessagePriceList(category, manufacturer, model string) (string, error) {

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
func MakeMessage_GetNewConfig(data CallBackData, UserId int64) string {
	message := fmt.Sprintf(
		"Добавление новой конфигурации\n"+
			"Введите данные в следующем формате:\n"+
			"```\n%s | %s | %s\nЦвет: Color, Страна: Country, Цена: Price, Количество: Quantity\n```",
		data.Category,
		data.Manufacturer,
		data.Model)

	return message
}

// Уникальные категории из каталога
func Get_categories() ([]string, error) {
	var categories []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("category").
		Order("category ASC").
		Pluck("category", &categories).
		Error
	return categories, err
}

// Выдает всех производителей для заданной категории
func Get_manufacturers(category string) ([]string, error) {
	var manufacturers []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("manufacturer").
		Where("category = ?", category).
		Order("manufacturer ASC").
		Pluck("manufacturer", &manufacturers).
		Error
	return manufacturers, err
}

// Выдает все модели для заданной категории и производиеля
func Get_device_models(category, manufacturer string) ([]string, error) {
	var res []string
	err := DB.Model(&models.CatalogItem{}).
		Distinct("device_model").
		Where("category = ? AND manufacturer = ?", category, manufacturer).
		Order("device_model ASC").
		Pluck("device_model", &res).
		Error
	return res, err
}

// (АДМИН) Отправка меню выбора действия в редактировании каталога
func GetRedactAction(userID int64, data CallBackData, botToken string) {

	keyboard := map[string]interface{}{
		"inline_keyboard": [][]map[string]string{},
		"resize_keyboard": true,
	}
	new_data := data
	new_data.Command = bot_commands.Redact_prices
	Redact_prices_button := map[string]string{
		"text":          "Редактировать цены",
		"callback_data": code_request(new_data),
	}
	newRow := []map[string]string{Redact_prices_button}

	rows := keyboard["inline_keyboard"].([][]map[string]string)
	keyboard["inline_keyboard"] = append(rows, newRow)

	new_data.Command = bot_commands.Redact_add_config
	Add_config_button := map[string]string{
		"text":          "Добавить конфигурацию",
		"callback_data": code_request(new_data),
	}

	newRow = []map[string]string{Add_config_button}

	rows = keyboard["inline_keyboard"].([][]map[string]string)
	keyboard["inline_keyboard"] = append(rows, newRow)

	new_data.Command = bot_commands.Redact_delete_config
	Delete_config_button := map[string]string{
		"text":          "Удалить конфигурацию",
		"callback_data": code_request(new_data),
	}

	newRow = []map[string]string{Delete_config_button}

	rows = keyboard["inline_keyboard"].([][]map[string]string)
	keyboard["inline_keyboard"] = append(rows, newRow)

	jsonData, _ := json.Marshal(map[string]interface{}{
		"chat_id":      userID,
		"text":         "Выберите действие:",
		"reply_markup": keyboard,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		log.Println("Ошибка отправки get redact action", err)
		return
	}
	defer resp.Body.Close()

}

// (АДМИН) Обработка CallBackQuery
func admin_callback_handler(query *CallbackQuery, botToken string) {
	fmt.Println("CALLBACK DATA:", query.Data)
	data := decode_request(query.Data)

	fmt.Println(data.Command)
	switch data.Command {
	case bot_commands.Start_redact: //Редактирование каталога(выбор девайса) -> меню с выбором категории
		categories, err := Get_categories()
		if err != nil {
			fmt.Println("Ошибка чтения category в БД")
		}
		ChooseDevice(categories, bot_commands.Choose_category, data, uint64(query.Message.Chat.ID))
		return

	case bot_commands.Start:
		{
			ShowStartMenu_admin(query.Message.Chat.ID)
			return
		}

	case bot_commands.Choose_category:
		{ //Выбор категории -> выбор производителя
			if data.Category == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			manufacturers, err := Get_manufacturers(data.Category)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			ChooseDevice(manufacturers, bot_commands.Choose_manufacturer, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_manufacturer:
		{ //Выбор производителя -> выбор модели

			if data.Category == "" || data.Manufacturer == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			device_models, err := Get_device_models(data.Category, data.Manufacturer)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			ChooseDevice(device_models, bot_commands.Choose_model, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_model:
		{ //выбор модели -> список всех конфигураций для выбранной модели
			//sendPrices(que)
			msg, err := MakeMessagePriceList(data.Category, data.Manufacturer, data.Model)
			if err != nil {
				fmt.Println("Ошибка извлечения записей из Products")
			} else {
				sendMessage(query.Message.Chat.ID, msg)
			}

			GetRedactAction(query.Message.Chat.ID, data, botToken)
			return
			// В окне редактирования сообщения
		}
	case bot_commands.Redact_prices:
		{
			data_msg, err := MakeMessage_GetConfigurationsForEdit(data.Category, data.Manufacturer, data.Model)
			if err != nil {
				fmt.Println("Ошибка создания списка для редактирования")
				return
			}
			sendMessage(query.Message.Chat.ID, data_msg)
			adminStates[query.Message.Chat.ID] = bot_commands.Wait_for_PriceList
		}

	case bot_commands.Redact_add_config:
		{
			msg_data := MakeMessage_GetNewConfig(data, query.Message.Chat.ID)
			sendMessage(query.Message.Chat.ID, msg_data)
			adminStates[query.Message.Chat.ID] = bot_commands.Wait_for_new_config
		}
	case bot_commands.Redact_delete_config:
		{
			sendMessage(query.Message.Chat.ID, "Введите ID удаляемой категории")
			adminStates[query.Message.Chat.ID] = bot_commands.Wait_for_delete_ID
		}

	default:
		sendMessage(query.Message.Chat.ID, "Unknown admin command")
		return
	}
}

// (АДМИН) Удаляет конфигурацию по ID
func removeProduct(msg *Message) {
	str := strings.TrimSpace(msg.Text)
	id, err := strconv.Atoi(str)
	if err != nil {
		sendMessage(msg.Chat.ID, "Некорректный ID")
		return
	}

	var product models.ProductInStock
	if err := DB.First(&product, id).Error; err != nil {
		sendMessage(msg.Chat.ID, fmt.Sprintf("Товар с ID %d не найден", id))
		return
	}

	if err := DB.Delete(&product).Error; err != nil {
		sendMessage(msg.Chat.ID, fmt.Sprintf("Ошибка при удалении товара с ID %d: %v", id, err))
		return
	}

	sendMessage(msg.Chat.ID, fmt.Sprintf("Товар с ID %d успешно удалён", id))
}

// Обработка сообщения редактирования списка конфигураций
func redactProductsInStock(msg *Message) (int, error) {
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

func userMessageHandler(msg *Message) {
	switch msg.Text {
	case "/start":
		ShowStartMenu_user(msg.Chat.ID)
		return
	default:
		//sendMessage(msg.Chat.ID)
		return
	}
}

// обработка сообщений админа
func admin_message_handler(msg *Message) {
	switch msg.Text {
	case "/start":
		ShowStartMenu_admin(msg.Chat.ID)
		return
	}

	switch adminStates[msg.Chat.ID] {
	case bot_commands.Wait_for_new_config:
		{
			new_conf, model, category, manufacturer, err := parseNewConfig(msg.Text)
			if err != nil {
				fmt.Println("Ошибка парсинга сообщения с новой конфигурацией", err)
				return
			}

			result := AddProductConfiguration(category, manufacturer, model, &new_conf)
			if result != nil {
				str := fmt.Sprintf("Config save error: %v", result)
				sendMessage(msg.Chat.ID, str)
			} else {
				sendMessage(msg.Chat.ID, "Конфигурация добавлена")
			}

			adminStates[msg.Chat.ID] = bot_commands.None
			ShowStartMenu_admin(msg.Chat.ID)
			return
		}
	case bot_commands.Wait_for_PriceList:
		{
			updated, err := redactProductsInStock(msg)
			if err != nil {
				sendMessage(msg.Chat.ID, fmt.Sprint("Ошибка обновления цен", err))
			} else {
				sendMessage(msg.Chat.ID, fmt.Sprintf("Успешно обновлено %d записей", updated))
			}
			adminStates[msg.Chat.ID] = bot_commands.None
			ShowStartMenu_admin(msg.Chat.ID)
		}

	case bot_commands.Wait_for_delete_ID:
		{
			removeProduct(msg)
			adminStates[msg.Chat.ID] = bot_commands.None
			ShowStartMenu_admin(msg.Chat.ID)
		}
	case bot_commands.None:
		sendMessage(msg.Chat.ID, "Unexpected message")
		return
	default:
		sendMessage(msg.Chat.ID, "Unexpected message")
		return

	}

}

func start_handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Can't read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var update Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.Println("JSON parse error:", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Обработка callback
	if update.CallbackQuery != nil {

		user_id := update.CallbackQuery.Message.Chat.ID
		if auth.GetAuth().IsAdmin(user_id) {
			admin_callback_handler(update.CallbackQuery, botToken)
		} else {
			user_callback_handler(update.CallbackQuery, botToken)
		}
		return
	}

	// Обработка сообщения
	var msg *Message
	if update.Message != nil {
		msg = update.Message
		user_id := msg.Chat.ID
		if auth.GetAuth().IsAdmin(user_id) {
			admin_message_handler(msg)
		} else {
			userMessageHandler(msg)
		}
		return
	} else if update.EditedMessage != nil {
		msg = update.EditedMessage
		user_id := msg.Chat.ID
		if auth.GetAuth().IsAdmin(user_id) {
			admin_message_handler(msg)
		} else {
			userMessageHandler(msg)
		}
		return

	}
	http.Error(w, "Unsupported update type", http.StatusBadRequest)

}

func SeedTestData() error {
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

func sendMessage(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshaling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Println("Error sending message:", err)
		return
	}
	defer resp.Body.Close()
}

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

func init() {
	var err error
	DB, err = Connect()
	if err != nil {
		log.Panic("Ошибка подключения к базе:", err)
	}
	DB.AutoMigrate(&models.CatalogItem{}, &models.Order{}, &models.ProductInStock{})
	DB.Migrator().CreateIndex(&models.CatalogItem{}, "idx_category_manufacturer_model") // Создает составной индекс из полей, помеченных тэгом
	SeedTestData()
	adminStates = auth.GetAuth().GetAdminStates()
	//DB.AutoMigrate(&models.User{}, &models.CatalogItem{}, &models.Transaction{})

	botToken = os.Getenv("BOT_TOKEN")
	webhook_url = os.Getenv("BOT_URL")
}

var botToken, webhook_url string

func main() {

	if len(os.Args) > 1 && os.Args[1] == "setwebhook" {
		setWebhook(webhook_url)
		fmt.Println(webhook_url)
		return
	}

	http.HandleFunc("/", start_handler)
	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
