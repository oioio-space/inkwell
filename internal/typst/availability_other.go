//go:build !linux

package typst

import "errors"

// Spec 9.11 refuses preview and export even for personal documents until N35.
func previewAvailable() error {
	return errors.New("aperçu indisponible sur ce système pour les documents reçus")
}
