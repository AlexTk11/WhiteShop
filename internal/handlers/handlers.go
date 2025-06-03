package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"tg_bot/auth"
	"tg_bot/internal/bot_commands"
	db "tg_bot/internal/database"
	menu "tg_bot/internal/keyboards"
	messages "tg_bot/internal/msg_gen"
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
	//code_request   = utils.Code_request
	decode_request = utils.Decode_request
)

var AdminStates map[int64]int = auth.GetAuth().GetAdminStates()

// (АДМИН) Обработка CallBackQuery
func admin_callback_handler(DB *gorm.DB, query *CallbackQuery, botToken string) {
	fmt.Println("CALLBACK DATA:", query.Data)
	data := decode_request(query.Data)

	fmt.Println(data.Command)
	switch data.Command {
	case bot_commands.Start:
		{

			menu.ShowStartMenu_admin(query.Message.Chat.ID)
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
			return
		}

	case bot_commands.Start_redact: //Редактирование каталога(выбор девайса) -> меню с выбором категории
		categories, err := db.Get_categories(DB)
		if err != nil {
			fmt.Println("Ошибка чтения category в БД")
		}
		menu.ChooseDevice(categories, bot_commands.Choose_category, data, uint64(query.Message.Chat.ID))
		utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		return

	case bot_commands.Choose_category:
		{ //Выбор категории -> выбор производителя
			if data.Category == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			manufacturers, err := db.Get_manufacturers(DB, data.Category)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			menu.ChooseDevice(manufacturers, bot_commands.Choose_manufacturer, data, uint64(query.Message.Chat.ID))
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
			return
		}
	case bot_commands.Choose_manufacturer:
		{ //Выбор производителя -> выбор модели

			if data.Category == "" || data.Manufacturer == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			device_models, err := db.Get_device_models(DB, data.Category, data.Manufacturer)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			menu.ChooseDevice(device_models, bot_commands.Choose_model, data, uint64(query.Message.Chat.ID))
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
			return
		}
	case bot_commands.Choose_model:
		{ //выбор модели -> список всех конфигураций для выбранной модели
			//sendPrices(que)
			msg, err := messages.MakeMessagePriceList(DB, data.Category, data.Manufacturer, data.Model)
			if err != nil {
				fmt.Println("Ошибка извлечения записей из Products")
			} else {
				utils.SendMessage(query.Message.Chat.ID, msg)
			}

			menu.GetRedactAction(query.Message.Chat.ID, data, botToken)
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
			return

		}
	case bot_commands.Redact_prices:
		{
			stock, err := db.Get_configurations(DB, data.Category, data.Manufacturer, data.Model)
			if err != nil {
				utils.SendMessage(query.Message.Chat.ID, fmt.Sprintf(
					"Ошибка загрузки конфигураций для модели %s %s %s",
					data.Category,
					data.Manufacturer,
					data.Model,
				))
				return
			}
			data_msg := messages.MakeMessage_GetConfigurationsForEdit(stock)
			utils.SendMessage(query.Message.Chat.ID, data_msg)
			AdminStates[query.Message.Chat.ID] = bot_commands.Wait_for_PriceList
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		}

	case bot_commands.Redact_add_config:
		{
			msg_data := messages.MakeMessage_GetNewConfig(data, query.Message.Chat.ID)
			utils.SendMessage(query.Message.Chat.ID, msg_data)
			AdminStates[query.Message.Chat.ID] = bot_commands.Wait_for_new_config
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		}
	case bot_commands.Redact_delete_config:
		{
			utils.SendMessage(query.Message.Chat.ID, "Введите ID удаляемой категории")
			AdminStates[query.Message.Chat.ID] = bot_commands.Wait_for_delete_ID
			utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		}

	default:
		utils.SendMessage(query.Message.Chat.ID, "Unknown admin command")
		utils.DeleteMessage(query.Message.Chat.ID, query.Message.MessageID)
		return
	}
}

// обработка сообщений админа
func admin_message_handler(DB *gorm.DB, msg *Message) {
	switch msg.Text {
	case "/start":
		menu.ShowStartMenu_admin(msg.Chat.ID)
		utils.DeleteMessage(msg.MessageID, msg.Chat.ID)
		return
	}

	switch AdminStates[msg.Chat.ID] {
	case bot_commands.Wait_for_new_config:
		{
			new_conf, model, category, manufacturer, err := messages.ParseNewConfig(msg.Text)
			if err != nil {
				fmt.Println("Ошибка парсинга сообщения с новой конфигурацией", err)
				return
			}

			result := db.AddProductConfiguration(DB, category, manufacturer, model, &new_conf)
			if result != nil {
				str := fmt.Sprintf("Config save error: %v", result)
				utils.SendMessage(msg.Chat.ID, str)
			} else {
				utils.SendMessage(msg.Chat.ID, "Конфигурация добавлена")
			}

			AdminStates[msg.Chat.ID] = bot_commands.None
			menu.ShowStartMenu_admin(msg.Chat.ID)
			return
		}
	case bot_commands.Wait_for_PriceList:
		{
			updated, err := db.RedactProductsInStock(DB, msg)
			if err != nil {
				utils.SendMessage(msg.Chat.ID, fmt.Sprint("Ошибка обновления цен", err))
			} else {
				utils.SendMessage(msg.Chat.ID, fmt.Sprintf("Успешно обновлено %d записей", updated))
			}
			AdminStates[msg.Chat.ID] = bot_commands.None
			menu.ShowStartMenu_admin(msg.Chat.ID)
		}

	case bot_commands.Wait_for_delete_ID:
		{
			err := db.RemoveProduct(DB, msg)
			if err != nil {
				utils.SendMessage(msg.Chat.ID, fmt.Sprint("Ошибка удаления конфигурации", err))
			} else {
				utils.SendMessage(msg.Chat.ID, "Запись успешно удалена")
			}
			AdminStates[msg.Chat.ID] = bot_commands.None
			menu.ShowStartMenu_admin(msg.Chat.ID)
		}
	case bot_commands.None:
		utils.SendMessage(msg.Chat.ID, "Unexpected message")
		return
	default:
		utils.SendMessage(msg.Chat.ID, "Unexpected message")
		return

	}

}

func userMessageHandler(msg *Message) {
	switch msg.Text {
	case "/start":
		menu.ShowStartMenu_user(msg.Chat.ID)
		return
	default:
		//sendMessage(msg.Chat.ID)
		return
	}
}

func StartHandler(DB *gorm.DB, w http.ResponseWriter, r *http.Request) {
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
			admin_callback_handler(DB, update.CallbackQuery, utils.BotToken)
		} else {
			userCallbackHandler(DB, update.CallbackQuery, utils.BotToken)
		}
		return
	}

	// Обработка сообщения
	var msg *Message
	if update.Message != nil {
		msg = update.Message
		user_id := msg.Chat.ID
		if auth.GetAuth().IsAdmin(user_id) {
			admin_message_handler(DB, msg)
		} else {
			userMessageHandler(msg)
		}
		return
	} else if update.EditedMessage != nil {
		msg = update.EditedMessage
		user_id := msg.Chat.ID
		if auth.GetAuth().IsAdmin(user_id) {
			admin_message_handler(DB, msg)
		} else {
			userMessageHandler(msg)
		}
		return

	}
	http.Error(w, "Unsupported update type", http.StatusBadRequest)

}

func userCallbackHandler(DB *gorm.DB, query *CallbackQuery, botToken string) {
	data := decode_request(query.Data)

	fmt.Println(data.Command)
	switch data.Command {

	case bot_commands.Start:
		{
			menu.ShowStartMenu_user(query.Message.Chat.ID)
			return
		}

	case bot_commands.Catalog_start: //Редактирование каталога(выбор девайса) -> меню с выбором категории
		{
			categories, err := db.Get_categories(DB)
			if err != nil {
				fmt.Println("Ошибка чтения category в БД")
			}
			menu.ChooseDevice(categories, bot_commands.Choose_category, data, uint64(query.Message.Chat.ID))
			return
		}

	case bot_commands.Choose_category:
		{ //Выбор категории -> выбор производителя
			if data.Category == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			manufacturers, err := db.Get_manufacturers(DB, data.Category)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			menu.ChooseDevice(manufacturers, bot_commands.Choose_manufacturer, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_manufacturer:
		{ //Выбор производителя -> выбор модели

			if data.Category == "" || data.Manufacturer == "" {
				fmt.Println("Error, get empty field in choose_model_command")
				return
			}
			device_models, err := db.Get_device_models(DB, data.Category, data.Manufacturer)
			if err != nil {
				fmt.Println("Ошибка чтения manufacturers в БД")
			}
			menu.ChooseDevice(device_models, bot_commands.Choose_model, data, uint64(query.Message.Chat.ID))
			return
		}
	case bot_commands.Choose_model:
		{ //выбор модели -> список всех конфигураций для выбранной модели
			//sendPrices(que)
			msg, err := messages.MakeMessagePriceList(DB, data.Category, data.Manufacturer, data.Model)
			if err != nil {
				fmt.Println("Ошибка извлечения записей из Products")
			} else {
				utils.SendMessage(query.Message.Chat.ID, msg)
			}

			menu.GetUserAction(DB, query.Message.Chat.ID, data, botToken)
			return
		}
	case bot_commands.MakeOrder: //Выбрал конфигурацию -> оформить заказ
		{
			var productID int
			var err error
			if productID, err = strconv.Atoi(data.ModelID); err != nil {
				fmt.Println("Некорректные данные в data.ModelID")
				utils.SendMessage(query.Message.Chat.ID, "Ошибка при создании заказа. Попробуйте позже")
				return
			}

			if err = db.MakeOrder(DB, query.Message.Chat.ID, uint(productID)); err != nil {
				fmt.Printf("Ошибка создания заказа %d", productID)
				utils.SendMessage(query.Message.Chat.ID, err.Error())
				return
			}
			// Рассылка админам нового заказа
			orderMessage := fmt.Sprintf("Новый заказ от пользователя @%s\n%s %s (ID: %d).", query.From.Username, data.Model, data.Manufacturer, productID)

			for admin := range AdminStates {
				utils.SendMessage(admin, orderMessage)
			}

			utils.SendMessage(query.Message.Chat.ID, "Заказ успешно оформлен!\n Скоро с вами свяжется менеджер для уточнения деталей.")
			return
		}

	default:
		utils.SendMessage(query.Message.Chat.ID, "Unknown user command")
	}

	fmt.Println(data.Command)
}
