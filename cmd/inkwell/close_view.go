package main

import (
	"context"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	savingLabel = "Enregistrement…"
	retryLabel  = "Réessayer"
	forceLabel  = "Fermer quand même"
)

type closeView struct {
	attempt uint32
	failure closeFrame
	all     bool
}

func watchWailsCloseDialog(ctx context.Context, app *application.App, window *application.WebviewWindow, closer *closeCoordinator) {
	go closer.watchView(ctx, window.ID(), func(view closeView) {
		if view.failure.Kind == "" {
			return
		}
		dialog := app.Dialog.Question().SetTitle("Enregistrement interrompu").SetMessage(view.message()).AttachToWindow(window)
		dialog.AddButton(retryLabel).SetAsDefault().OnClick(func() { closer.retry(view.attempt) })
		dialog.AddButton(forceLabel).OnClick(func() { closer.force(view.attempt) })
		dialog.AddButton("Annuler").SetAsCancel().OnClick(func() { closer.dismiss(view.attempt) })
		dialog.Show()
	})
}

func (c *closeCoordinator) watchView(ctx context.Context, window uint, render func(closeView)) {
	var previous closeView
	for {
		c.mu.Lock()
		changed := c.changed
		var view closeView
		if a := c.current; a != nil && !a.approved && a.cuts[window] != nil {
			view = closeView{attempt: a.id, failure: a.failure, all: a.window == 0}
		}
		c.mu.Unlock()
		if view.attempt != previous.attempt || view.failure.Kind != previous.failure.Kind {
			render(view)
			previous = view
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return
		}
	}
}

func (v closeView) message() string {
	lines := []string{"Les dernières modifications de ce projet n’ont pas pu être vérifiées."}
	if names := v.failure.Files; len(names) > 0 {
		lines = []string{"Modifications à vérifier dans les fichiers suivants", strings.Join(names, ", ")}
	}
	if v.failure.Reason == "disk-full" {
		lines = append(lines, "Libérez de l’espace, puis réessayez.")
	}
	lines = append(lines, "Rouvrez le même projet sur cet appareil pour retrouver les modifications déjà enregistrées.") //nolint:misspell // French interface wording.
	if v.failure.Reason == "crashed" {
		lines = append(lines, "Les modifications qui étaient encore uniquement dans la fenêtre ont été perdues.")
	} else {
		lines = append(lines, "Si le texte est encore visible, vous pouvez le copier en revenant à l’éditeur.")
	}
	if v.all {
		lines = append(lines, "« Fermer quand même » ferme toutes les fenêtres d’Inkwell.")
	}
	return strings.Join(lines, "\n\n")
}
