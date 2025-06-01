package models

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	EditedMessage *Message       `json:"edited_message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	Text string `json:"text"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

type CallbackQuery struct {
	From struct {
		Username string `json:"username"`
	} `json:"from"`
	Message struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
	Data string `json:"data"`
}

type CallBackData struct {
	Command      string `json:"com"`
	Category     string `json:"cat"`
	Manufacturer string `json:"man"`
	Model        string `json:"mod"`
	ModelID      string `json:"mID"`
}
