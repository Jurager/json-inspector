package wails

import "github.com/wailsapp/wails/v3/pkg/application"

// MenuLabels are the few words the native menu needs. They arrive from the window, because the menu is
// drawn by the system and only the page knows the language — the alternative would be a second
// catalogue here, in one language, drifting away from the one the rest of the app reads.
type MenuLabels struct {
	About        string `json:"about"`
	Help         string `json:"help"`
	CheckUpdates string `json:"checkUpdates"`
}

// BuildMenu is the native menu for the platforms that do not draw their own title bar. It is built
// again when the language is settled, which is why it takes the labels rather than reading them.
func BuildMenu(host *Host, name string, labels MenuLabels) *application.Menu {
	menu := application.NewMenu()

	appMenu := menu.AddSubmenu(name)
	appMenu.Add(labels.About).OnClick(func(*application.Context) { host.ShowAbout() })
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	helpMenu := menu.AddSubmenu(labels.Help)
	helpMenu.Add(labels.CheckUpdates).
		SetAccelerator("CmdOrCtrl+U").
		OnClick(func(*application.Context) { host.RequestUpdateCheck() })

	// The stock Edit and Window menus bring the standard roles along with them.
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.WindowMenu)

	return menu
}
