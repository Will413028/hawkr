package main

import (
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

func modules() []migrate.Module {
	return []migrate.Module{
		{Name: "content", Dir: content.Migrations()},
		{Name: "identity", Dir: identity.Migrations()},
		{Name: "media", Dir: media.Migrations()},
		{Name: "delegation", Dir: delegation.Migrations()},
		{Name: "buyer", Dir: buyer.Migrations()},
		{Name: "accountdeletion", Dir: accountdeletion.Migrations()},
		{Name: "seller", Dir: seller.Migrations()},
		{Name: "discovery", Dir: discovery.Migrations()},
		{Name: "social", Dir: social.Migrations()},
		{Name: "payment", Dir: payment.Migrations()},
		{Name: "moderation", Dir: moderation.Migrations()},
		{Name: "assistant", Dir: assistant.Migrations()},
		{Name: "procurement", Dir: procurement.Migrations()},
		{Name: "review", Dir: review.Migrations()},
		{Name: "im", Dir: im.Migrations()},
		{Name: "crm", Dir: crm.Migrations()},
		{Name: "coupon", Dir: coupon.Migrations()},
		{Name: "imageai", Dir: imageai.Migrations()},
		{Name: "notification", Dir: notification.Migrations()},
		{Name: "catalog", Dir: catalog.Migrations()},
	}
}

func platformData() []migrate.Module {
	return []migrate.Module{{Name: inbox.Schema, Dir: inbox.Migrations()}, {Name: adminaudit.Schema, Dir: adminaudit.Migrations()}, {Name: bizcalendar.Schema, Dir: bizcalendar.Migrations()}}
}

func migrations() []migrate.Module { return append(platformData(), modules()...) }

var apiOnly = []string{}

var withoutData = []string{
	"reviewqueue",
	"accountsuspension",
}
