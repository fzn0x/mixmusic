//go:build !(linux && (amd64 || arm64))

package portal

import "context"

func requestBackground(context.Context, Background) (Answer, error) {
	return Answer{}, ErrUnsupported
}
