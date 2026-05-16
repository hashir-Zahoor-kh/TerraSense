package store

import "encoding/json"

// jsonUnmarshal is a thin alias so store files don't each import encoding/json directly.
var jsonUnmarshal = json.Unmarshal
