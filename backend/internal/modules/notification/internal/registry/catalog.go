package registry

import "slices"

var all = slices.Concat(accountTypes, announcementTypes, contentTypes, governanceTypes, couponTypes)
