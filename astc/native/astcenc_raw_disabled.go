//go:build !astcenc_native

package native

import "github.com/arm-software/astc-encoder/astc"

func ConfigInit(profile astc.Profile, blockX, blockY, blockZ int, quality float32, flags Flags) (Config, error) {
	return Config{}, errDisabled
}

func ContextAlloc(cfg *Config, threadCount int) (*Context, error) {
	return nil, errDisabled
}

func (c *Context) Close() error { return errDisabled }

func (c *Context) CompressImage(img *Image, swizzle Swizzle, out []byte, threadIndex int) error {
	return errDisabled
}

func (c *Context) CompressReset() error  { return errDisabled }
func (c *Context) CompressCancel() error { return errDisabled }

func (c *Context) DecompressImage(data []byte, imgOut *Image, swizzle Swizzle, threadIndex int) error {
	return errDisabled
}

func (c *Context) DecompressReset() error { return errDisabled }

func (c *Context) GetBlockInfo(block [astc.BlockBytes]byte) (BlockInfo, error) {
	return BlockInfo{}, errDisabled
}
