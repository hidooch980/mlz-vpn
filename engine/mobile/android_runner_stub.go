//go:build !android

package mobile

import (
	"context"
	"errors"

	"github.com/mlz-vpn/mlz-vpn/engine/pkg/orchestrator"
)

type unsupportedAndroidRunner struct{}

func newAndroidRunner(_ string, _ int32, _ SocketProtector, _ string) orchestrator.CoreRunner {
	return &unsupportedAndroidRunner{}
}

func (a *unsupportedAndroidRunner) StartSingBox(_ context.Context, _ []byte) error {
	return errors.New("android dataplane runtime is not bundled in this build")
}
func (a *unsupportedAndroidRunner) StopSingBox() error { return nil }
func (a *unsupportedAndroidRunner) StartXray(_ context.Context, _ []byte) error {
	return errors.New("xray dataplane is not bundled on Android; use sing-box")
}
func (a *unsupportedAndroidRunner) StopXray() error { return nil }
