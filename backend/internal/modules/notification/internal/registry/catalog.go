package registry

import "slices"

// all 由各領域檔的型別清單串成，順序固定：新增型別只改自己領域檔的清單，這裡不動。
var all = slices.Concat(accountTypes, announcementTypes, contentTypes, governanceTypes)
