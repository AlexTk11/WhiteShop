package bot_commands

const ( //CallbackQuery commands

	Start = "0"
	//----------------admin---------------
	Choose_category      = "1"
	Choose_manufacturer  = "2"
	Choose_model         = "3"
	Start_redact         = "4"
	Redact_add_config    = "5"
	Redact_prices        = "6"
	Redact_delete_config = "7"

	//----------------user---------------
	Catalog_start = "20"
	MakeOrder     = "21"
	CheckOrder    = "22"

// -------------------all-------------------
)

const ( //admin_states
	None                = 0
	Wait_for_new_config = iota
	Wait_for_PriceList
	Wait_for_delete_ID
)
