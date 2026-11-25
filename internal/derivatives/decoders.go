package derivatives

// Every image.Decode in this package — thumbnail warming and the originals
// audit — sees exactly the format set registered here. The audit's exclude
// mode marks rows failed when decode fails, so this set must cover every
// format uploads and connectors legitimately store as originals; a decoder
// missing here turns valid pieces into "truncated-or-corrupt" exclusions.
import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)
