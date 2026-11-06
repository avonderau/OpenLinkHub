package pluginhost

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// An install from the editor arrives over the network side of OpenLinkHub,
// and the plugin then runs as the desktop user. The desktop user confirms
// every such install in a dialog on their own screen, so another program
// or user that can reach the WebUI cannot install code that runs as them.

const confirmTimeout = 90 * time.Second

// ErrInstallDeclined is returned when the desktop user said no or did not
// answer.
var ErrInstallDeclined = errors.New("the install was not confirmed on the desktop")

// confirmDialog is replaced in tests.
var confirmDialog = runConfirmDialog

// ConfirmInstall asks the desktop user to approve installing plugin.
func ConfirmInstall(plugin *Plugin) error {
	return confirmDialog(installQuestion(plugin))
}

func installQuestion(p *Plugin) string {
	author := p.Author
	if author == "" {
		author = "unknown author"
	}
	actions := make([]string, 0, len(p.Actions))
	for i, a := range p.Actions {
		if i == 5 {
			actions = append(actions, fmt.Sprintf("and %d more", len(p.Actions)-5))
			break
		}
		actions = append(actions, a.Name)
	}
	text := fmt.Sprintf("Install the Deck plugin \"%s\" %s by %s?\n\nID: %s", p.Name, p.Version, author, p.UUID)
	if len(actions) > 0 {
		text += "\nActions: " + strings.Join(actions, ", ")
	}
	text += "\n\nThis was requested from the OpenLinkHub Deck editor. The plugin is a program that will run as your user. Install it only if you started this install and trust the plugin."
	return plainText(text)
}

// plainText strips control characters a manifest could use to disguise
// the question; the dialogs are also told not to interpret markup.
func plainText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			return -1
		}
		return r
	}, s)
}

func runConfirmDialog(question string) error {
	ctx, cancel := context.WithTimeout(context.Background(), confirmTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if path, err := exec.LookPath("zenity"); err == nil {
		cmd = exec.CommandContext(ctx, path, "--question", "--no-markup", "--title=OpenLinkHub: install Deck plugin",
			"--ok-label=Install", "--cancel-label=Cancel", "--width=420", "--text="+question)
	} else if path, err := exec.LookPath("kdialog"); err == nil {
		// kdialog treats text that looks like HTML as rich text; escape it.
		escaped := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(question)
		cmd = exec.CommandContext(ctx, path, "--title", "OpenLinkHub: install Deck plugin", "--yesno", escaped)
	} else {
		return errors.New("cannot ask for confirmation: install zenity (or kdialog), or install the plugin from a terminal with OpenLinkHub --install-deck-plugin")
	}
	if err := cmd.Run(); err != nil {
		return ErrInstallDeclined
	}
	return nil
}
