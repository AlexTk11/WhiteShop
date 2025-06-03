package menu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"tg_bot/internal/bot_commands"
	"tg_bot/internal/utils"
	"tg_bot/models"

	"gorm.io/gorm"
)

type (
	CallbackQuery = models.CallbackQuery
	CallBackData  = models.CallBackData
	Message       = models.Message
	Update        = models.Update
)

var (
	code_request = utils.Code_request
	//decode_request = utils.Decode_request
)

var botToken = os.Getenv("BOT_TOKEN")

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
func GetUserAction(DB *gorm.DB, userID int64, data CallBackData, botToken string) error {

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

// (АДМИН) Отправляет стартовое меню
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
