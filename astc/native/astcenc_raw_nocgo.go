//go:build astcenc_native && !cgo

package native

import "github.com/arm-software/astc-encoder/astc"

func ConfigInit(profile astc.Profile, blockX, blockY, blockZ int, quality float32, flags Flags) (Config, error) {
	return Config{}, errNoCGO
}

func ContextAlloc(cfg *Config, threadCount int) (*Context, error) {
	return nil, errNoCGO
}

func (c *Context) Close() error { return errNoCGO }

func (c *Context) CompressImage(img *Image, swizzle Swizzle, out []byte, threadIndex int) error {
	return errNoCGO
}

func (c *Context) CompressReset() error  { return errNoCGO }
func (c *Context) CompressCancel() error { return errNoCGO }

func (c *Context) DecompressImage(data []byte, imgOut *Image, swizzle Swizzle, threadIndex int) error {
	return errNoCGO
}

func (c *Context) DecompressReset() error { return errNoCGO }

func (c *Context) GetBlockInfo(block [astc.BlockBytes]byte) (BlockInfo, error) {
	return BlockInfo{}, errNoCGO
}
