package store

import "encoding/json"

// jsonUnmarshal and jsonMarshal are thin aliases so store files don't each import encoding/json directly.
var jsonUnmarshal = json.Unmarshal
var jsonMarshal = json.Marshal
