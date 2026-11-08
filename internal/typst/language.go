package typst

// LanguagePaths returns the same pinned package view and font directories used
// for compilation. The caller keeps cleanup alive until its sidecar exits.
// Project packages cannot shadow bundled packages of the same name/version.
func (e *Engine) LanguagePaths(root string) (packages string, fonts, readPaths []string, cleanup func(), err error) {
	packages, cleanup, err = e.packageView(root)
	if err != nil {
		return "", nil, nil, nil, err
	}
	_, err = e.fonts.Ensure()
	if err != nil {
		cleanup()
		return "", nil, nil, nil, err
	}
	return packages, []string{e.fonts.root}, []string{e.fonts.root, e.pkgs.root, packages}, cleanup, nil
}
