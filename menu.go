package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
)

func buildMenu(app *App) *menu.Menu {
	// The app menu is the standard macOS one (menu.AppMenu) rather than a
	// hand-built list: Wails only contributes the native "About <app>" item —
	// the system panel configured in main.go — when the menu comes from that
	// role. Hide and Quit come with it.
	help := &menu.MenuItem{
		Label: "Справка",
		SubMenu: menu.NewMenuFromItems(
			&menu.MenuItem{
				Label:       "Проверить обновления…",
				Accelerator: keys.CmdOrCtrl("u"),
				Click:       func(*menu.CallbackData) { app.checkForUpdatesFromMenu() },
			},
		),
	}

	return menu.NewMenuFromItems(menu.AppMenu(), help, menu.EditMenu(), menu.WindowMenu())
}
