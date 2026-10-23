//go:build !vips

package media

import "errors"

var errVipsUnavailable = errors.New("govips disabled: build with -tags vips and install libvips-dev (CGO_ENABLED=1)")

// InitVips is a no-op when built without CGO.
func InitVips() error {
	return errVipsUnavailable
}

// ShutdownVips is a no-op when built without CGO.
func ShutdownVips() {}

func vipsDownscaleMaxLongest(_ []byte, _ int) ([]byte, error) {
	return nil, errVipsUnavailable
}

func vipsCoverExact(_ []byte, _, _ int) ([]byte, error) {
	return nil, errVipsUnavailable
}

func vipsScaleToEdgeRange(_ []byte, _, _ int) ([]byte, error) {
	return nil, errVipsUnavailable
}

func vipsDeliverResize(_ []byte, _, _, _ int) ([]byte, string, error) {
	return nil, "", errVipsUnavailable
}

func vipsDownscaleMaxBox(_ []byte, _, _ int) ([]byte, error) {
	return nil, errVipsUnavailable
}
