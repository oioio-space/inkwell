package bypass

import "wrapped" // want "forbidden import wrapped"

var indirect = wrapped.Launch
