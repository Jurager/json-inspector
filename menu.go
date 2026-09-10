package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func buildMenu(app *App) *menu.Menu {
	appMenu := &menu.MenuItem{
		Label: "JSON Inspector",
		SubMenu: menu.NewMenuFromItems(
			&menu.MenuItem{
				Label: "О программе",
				Click: func(*menu.CallbackData) { app.ShowAbout() },
			},
			menu.Separator(),
			&menu.MenuItem{
				Label:       "Проверить обновления…",
				Accelerator: keys.CmdOrCtrl("u"),
				Click:       func(*menu.CallbackData) { app.checkForUpdatesFromMenu() },
			},
			menu.Separator(),
			&menu.MenuItem{
				Label:       "Завершить JSON Inspector",
				Accelerator: keys.CmdOrCtrl("q"),
				Click:       func(*menu.CallbackData) { runtime.Quit(app.ctx) },
			},
		),
	}

	return menu.NewMenuFromItems(appMenu, menu.EditMenu(), menu.WindowMenu())
}
