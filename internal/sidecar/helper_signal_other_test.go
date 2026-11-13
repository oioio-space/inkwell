//go:build !linux

package sidecar

func ignoreFileSizeSignal() {}

func capWriteHelper(string) int { return 2 }

func lowerCoreHelper(string) int { return 1 }

func scopeAttackHelper(string, string) int { return 2 }
