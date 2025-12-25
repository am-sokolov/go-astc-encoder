package astc

// Profile controls decoding behavior for ASTC endpoints.
//
// Note: ASTC files do not store a profile; it is a usage convention. The
// reference astcenc library requires the caller to specify it.
type Profile uint8

const (
	// ProfileLDR decodes using linear LDR rules.
	ProfileLDR Profile = iota
	// ProfileLDRSRGB decodes using sRGB LDR rules.
	ProfileLDRSRGB
	// ProfileHDRRGBLDRAlpha decodes using HDR RGB and LDR alpha rules.
	ProfileHDRRGBLDRAlpha
	// ProfileHDR decodes using HDR RGBA rules.
	ProfileHDR
)
