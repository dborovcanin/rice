package config

// DefaultConfig returns a complete, working configuration. It is the single
// source of truth for defaults: `rice init` writes it out, and loading a
// partial config.toml decodes on top of it.
func DefaultConfig() Config {
	return Config{
		Theme: "gruvbox-dark",
		Components: Components{
			Sway:     true,
			Waybar:   true,
			Rofi:     true,
			Foot:     true,
			Dunst:    true,
			Swaylock: true,
			GTK:      true,
			Qt:       true,
		},
		Generations: Generations{Keep: 10},
		Commands:    defaultCommands(),
		Sway:        defaultSway(),
		Waybar:      defaultWaybar(),
		Rofi:        defaultRofi(),
		Foot:        defaultFoot(),
		Dunst:       defaultDunst(),
		Swaylock:    defaultSwaylock(),
		GTK:         defaultGTK(),
		Qt:          defaultQt(),
	}
}

func defaultGTK() GTK {
	return GTK{
		Settings: true,
		// The palette mapping is off by default: it is the one part of
		// toolkit integration that changes how applications look rather than
		// only which theme they load, and a GTK application that fights its
		// own theme looks worse than one that ignores Rice.
		CSS: false,
	}
}

func defaultQt() Qt {
	return Qt{
		Qt5ct:         true,
		Qt6ct:         true,
		Kvantum:       true,
		PlatformTheme: "qt5ct",
	}
}

func defaultCommands() Commands {
	return Commands{
		Terminal:       "footclient",
		TerminalSpawn:  "foot",
		Launcher:       "rofi -show drun",
		Browser:        "brave",
		Editor:         "codium",
		Lock:           "swaylock",
		Screenshot:     "grim -g \"$(slurp)\" - | wl-copy",
		Clipboard:      "cliphist list | rofi -dmenu | cliphist decode | wl-copy",
		Emoji:          "",
		VolumeUp:       "wpctl set-volume -l 1.5 @DEFAULT_AUDIO_SINK@ 5%+",
		VolumeDown:     "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-",
		VolumeMute:     "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle",
		MicMute:        "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle",
		BrightnessUp:   "brightnessctl set 5%+",
		BrightnessDown: "brightnessctl set 5%-",
	}
}

func defaultSway() Sway {
	return Sway{
		WriteEnvironment:  true,
		Mod:               "Mod4",
		WallpaperMode:     "fill",
		SmartBorders:      true,
		SmartGaps:         true,
		FocusFollowsMouse: false,
		Titlebar:          false,

		Outputs: []Output{
			{Name: "eDP-1", Resolution: "1920x1200", Position: "0 0", Scale: 1},
			{Name: "DP-1", Position: "1920 0", Scale: 1, Transform: "normal"},
			{Name: "HDMI-A-1", Position: "1920 0", Scale: 1},
		},

		Workspaces: []Workspace{
			{Key: "1", Name: "1"},
			{Key: "2", Name: "2"},
			{Key: "3", Name: "3"},
			{Key: "4", Name: "4"},
			{Key: "5", Name: "5"},
			{Key: "6", Name: "6"},
			{Key: "7", Name: "7"},
			{Key: "8", Name: "8"},
			{Key: "9", Name: "9"},
			{Key: "0", Name: "10"},
		},

		Bindings: defaultBindings(),
		Modes:    defaultModes(),

		WindowRules: []WindowRule{
			{Criteria: `[app_id="sticky_term"]`, Commands: "floating enable, sticky enable, resize set 1400 1000, move position center"},
			{Criteria: `[app_id="content-search"]`, Commands: "floating enable, resize set 1500 950, move position center"},
			{Criteria: `[app_id="file-search"]`, Commands: "floating enable, resize set 1500 950, move position center"},
			{Criteria: `[app_id="system-monitor"]`, Commands: "floating enable, resize set 1800 900, move position center"},
			{Criteria: `[app_id="satty"]`, Commands: "floating enable"},
			{Criteria: `[title=".*Meet.*"]`, Commands: "inhibit_idle visible, sticky enable"},
		},

		Assigns: []Assign{
			{Criteria: `[app_id="Slack"]`, Workspace: "1"},
			{Criteria: `[class="Slack"]`, Workspace: "1"},
			{Criteria: `[app_id="codium"]`, Workspace: "2"},
			{Criteria: `[class="VSCodium"]`, Workspace: "2"},
			{Criteria: `[app_id="brave-browser"]`, Workspace: "3"},
			{Criteria: `[class="Postman"]`, Workspace: "4"},
			{Criteria: `[class="zoom"]`, Workspace: "6"},
		},

		Startup: []StartupItem{
			{Command: "dbus-update-activation-environment --systemd WAYLAND_DISPLAY SWAYSOCK XDG_CURRENT_DESKTOP=sway", Always: false, Comment: "export session environment"},
			{Command: "wl-paste --watch cliphist store --max-items=5000", Always: false, Comment: "clipboard history"},
			{Command: "kdeconnectd", Always: false},
		},

		Keyboard: Keyboard{
			Layout:  "us,rs,rs",
			Variant: ",latin,",
			Options: "grp:alt_shift_toggle",
		},
		Touchpad: Touchpad{
			Tap:                true,
			TapButtonMap:       "lrm",
			DisableWhileTyping: true,
			MiddleEmulation:    true,
			DragLock:           false,
		},
		Idle: Idle{
			Enabled:     true,
			LockAfter:   300,
			ScreenOff:   600,
			LockOnSleep: true,
		},
	}
}

// defaultBindings mirrors the bindings from the author's Sway setup, with
// focus and move normalized onto vim keys so the two agree.
func defaultBindings() []Binding {
	return []Binding{
		{Keys: "$mod+Return", Command: "exec $terminal", Comment: "terminal"},
		{Keys: "$mod+Shift+Return", Command: "exec $terminal --app-id sticky_term", Comment: "floating sticky terminal"},
		{Keys: "$mod+Shift+q", Command: "kill", Comment: "close window"},
		{Keys: "$mod+d", Command: "exec $launcher", Comment: "application launcher"},

		{Keys: "$mod+h", Command: "focus left"},
		{Keys: "$mod+j", Command: "focus down"},
		{Keys: "$mod+k", Command: "focus up"},
		{Keys: "$mod+l", Command: "focus right"},
		{Keys: "$mod+Left", Command: "focus left"},
		{Keys: "$mod+Down", Command: "focus down"},
		{Keys: "$mod+Up", Command: "focus up"},
		{Keys: "$mod+Right", Command: "focus right"},

		{Keys: "$mod+Shift+h", Command: "move left"},
		{Keys: "$mod+Shift+j", Command: "move down"},
		{Keys: "$mod+Shift+k", Command: "move up"},
		{Keys: "$mod+Shift+l", Command: "move right"},
		{Keys: "$mod+Shift+Left", Command: "move left"},
		{Keys: "$mod+Shift+Down", Command: "move down"},
		{Keys: "$mod+Shift+Up", Command: "move up"},
		{Keys: "$mod+Shift+Right", Command: "move right"},

		{Keys: "$mod+backslash", Command: "split h", Comment: "split horizontally"},
		{Keys: "$mod+minus", Command: "split v", Comment: "split vertically"},
		{Keys: "$mod+f", Command: "fullscreen toggle"},
		{Keys: "$mod+w", Command: "layout tabbed"},
		{Keys: "$mod+s", Command: "layout toggle split"},
		{Keys: "$mod+t", Command: "floating toggle"},
		{Keys: "$mod+space", Command: "focus mode_toggle"},
		{Keys: "$mod+a", Command: "focus parent"},
		{Keys: "$mod+x", Command: "[urgent=latest] focus", Comment: "jump to urgent window"},

		{Keys: "$mod+m", Command: "scratchpad show"},
		{Keys: "$mod+Shift+m", Command: "move scratchpad"},

		{Keys: "$mod+g", Command: "exec $browser"},
		{Keys: "$mod+b", Command: "exec $terminal --app-id system-monitor btop"},
		{Keys: "$mod+c", Command: "exec $clipboard", Comment: "clipboard history"},
		{Keys: "$mod+Shift+s", Command: "exec $screenshot", Comment: "screenshot region"},
		{Keys: "$mod+Shift+c", Command: "exec $editor $HOME/.config/rice"},

		{Keys: "$mod+Shift+r", Command: "reload", Comment: "reload sway"},
		{Keys: "$mod+Shift+e", Command: `exec swaynag -t warning -m 'Exit Sway?' -B 'Exit' 'swaymsg exit'`},

		{Keys: "XF86AudioRaiseVolume", Command: "exec $volume_up"},
		{Keys: "XF86AudioLowerVolume", Command: "exec $volume_down"},
		{Keys: "XF86AudioMute", Command: "exec $volume_mute"},
		{Keys: "XF86AudioMicMute", Command: "exec $mic_mute"},
		{Keys: "XF86MonBrightnessUp", Command: "exec $brightness_up"},
		{Keys: "XF86MonBrightnessDown", Command: "exec $brightness_down"},

		{Keys: "Ctrl+space", Command: "exec dunstctl close", Comment: "dismiss notification"},
		{Keys: "Ctrl+Shift+space", Command: "exec dunstctl close-all"},
		{Keys: "Mod1+grave", Command: "exec dunstctl history-pop"},
		{Keys: "Ctrl+Shift+period", Command: "exec dunstctl context"},
	}
}

func defaultModes() []Mode {
	return []Mode{
		{
			Name:  "resize",
			Enter: "$mod+r",
			Bindings: []Binding{
				{Keys: "h", Command: "resize shrink width 10 px or 10 ppt"},
				{Keys: "j", Command: "resize grow height 10 px or 10 ppt"},
				{Keys: "k", Command: "resize shrink height 10 px or 10 ppt"},
				{Keys: "l", Command: "resize grow width 10 px or 10 ppt"},
				{Keys: "Left", Command: "resize shrink width 10 px or 10 ppt"},
				{Keys: "Down", Command: "resize grow height 10 px or 10 ppt"},
				{Keys: "Up", Command: "resize shrink height 10 px or 10 ppt"},
				{Keys: "Right", Command: "resize grow width 10 px or 10 ppt"},
			},
		},
		{
			Name:  "gaps",
			Enter: "$mod+Shift+g",
			Bindings: []Binding{
				{Keys: "plus", Command: "gaps inner current plus 5"},
				{Keys: "minus", Command: "gaps inner current minus 5"},
				{Keys: "0", Command: "gaps inner current set 0"},
				{Keys: "Shift+plus", Command: "gaps inner all plus 5"},
				{Keys: "Shift+minus", Command: "gaps inner all minus 5"},
				{Keys: "Shift+0", Command: "gaps inner all set 0"},
			},
		},
		{
			Name:  "move_workspace",
			Enter: "$mod+o",
			Bindings: []Binding{
				{Keys: "Left", Command: "move workspace to output left"},
				{Keys: "Down", Command: "move workspace to output down"},
				{Keys: "Up", Command: "move workspace to output up"},
				{Keys: "Right", Command: "move workspace to output right"},
			},
		},
		{
			Name:  "system",
			Enter: "$mod+Escape",
			Bindings: []Binding{
				{Keys: "l", Command: "exec $lock"},
				{Keys: "e", Command: "exec swaymsg exit"},
				{Keys: "s", Command: "exec systemctl suspend"},
				{Keys: "h", Command: "exec systemctl hibernate"},
				{Keys: "r", Command: "exec systemctl reboot"},
				{Keys: "Shift+s", Command: "exec systemctl poweroff"},
			},
		},
	}
}

func defaultWaybar() Waybar {
	return Waybar{
		Position:      "top",
		Layer:         "top",
		Design:        DefaultWaybarDesign,
		Height:        32,
		Spacing:       4,
		ModulesLeft:   []string{"sway/workspaces", "sway/mode"},
		ModulesCenter: []string{"sway/window"},
		ModulesRight:  []string{"pulseaudio", "cpu", "memory", "disk", "battery", "clock", "tray"},
		Modules: map[string]map[string]any{
			"sway/workspaces": {
				"disable-scroll": true,
				"all-outputs":    false,
			},
			"sway/window": {
				"max-length": 60,
			},
			"clock": {
				"format":         " {:%H:%M}",
				"format-alt":     " {:%a %d %b %Y}",
				"tooltip-format": "<tt>{calendar}</tt>",
			},
			"cpu": {
				"format":   " {usage}%",
				"interval": 5,
			},
			"memory": {
				"format":   " {percentage}%",
				"interval": 5,
			},
			"disk": {
				"format":   " {percentage_used}%",
				"path":     "/",
				"interval": 60,
			},
			"battery": {
				"format":          " {capacity}%",
				"format-charging": " {capacity}%",
				"format-icons":    []any{"", "", "", "", ""},
				"states":          map[string]any{"warning": 30, "critical": 15},
			},
			"pulseaudio": {
				"format":       " {volume}%",
				"format-muted": " muted",
				"on-click":     "pavucontrol",
				"scroll-step":  5,
			},
			"tray": {
				"spacing": 8,
			},
		},
	}
}

func defaultRofi() Rofi {
	return Rofi{
		Width:       "30%",
		Lines:       10,
		Columns:     1,
		ShowIcons:   true,
		Modes:       []string{"drun", "run", "window"},
		DisplayDrun: "Run",
	}
}

func defaultFoot() Foot {
	return Foot{
		Server:          true,
		Term:            "foot",
		ScrollbackLines: 250000,
		PadX:            8,
		PadY:            8,
		CursorStyle:     "beam",
		CursorBlink:     true,
	}
}

func defaultDunst() Dunst {
	return Dunst{
		Origin:          "top-center",
		Width:           420,
		Height:          200,
		Offset:          "0x10",
		GapSize:         10,
		Follow:          "mouse",
		TimeoutLow:      5,
		TimeoutNormal:   5,
		TimeoutCritical: 0,
		MaxIconSize:     64,
	}
}

func defaultSwaylock() Swaylock {
	return Swaylock{
		ScalingMode:        "fill",
		IndicatorRadius:    100,
		IndicatorThickness: 8,
		ShowFailed:         true,
		Clock:              true,
	}
}
