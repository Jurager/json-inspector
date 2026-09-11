package main

import "github.com/wailsapp/wails/v3/pkg/application"

func buildMenu(a *App) *application.Menu {
	menu := application.NewMenu()

	appMenu := menu.AddSubmenu(appName)
	appMenu.Add("О программе").OnClick(func(*application.Context) { a.ShowAbout() })
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	helpMenu := menu.AddSubmenu("Справка")
	helpMenu.Add("Проверить обновления…").
		SetAccelerator("CmdOrCtrl+U").
		OnClick(func(*application.Context) { a.checkForUpdatesFromMenu() })

	// The stock Edit and Window menus bring the standard roles along with them.
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.WindowMenu)

	return menu
}
