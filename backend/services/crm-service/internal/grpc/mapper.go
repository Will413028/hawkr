package grpc

import (
	"strconv"

	"github.com/Will413028/hawkr/crm-service/internal/usecase/dto"
	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
)

// toCrmCustomerProto converts a dto.Customer to pb.CrmCustomer
func toCrmCustomerProto(c *dto.Customer) *pb.CrmCustomer {
	if c == nil {
		return nil
	}
	proto := &pb.CrmCustomer{
		Id:                 strconv.FormatInt(c.ID, 10),
		StoreId:            strconv.FormatInt(c.StoreID, 10),
		BuyerId:            strconv.FormatInt(c.BuyerID, 10),
		CustomerType:       c.CustomerType,
		TotalSpent:         strconv.FormatInt(c.TotalSpent, 10),
		OrderCount:         strconv.Itoa(c.OrderCount),
		Nickname:           c.Nickname,
		AvatarKey:          c.Avatar, // 49 §6.6 S9：透傳 objectKey；Gateway 組 avatarUrl
		Phone:              c.Phone,
		Address:            c.Address,
		Email:              c.Email,
		CouponCount:        strconv.FormatInt(int64(c.CouponCount), 10),
		FrequentCategories: c.FrequentCategories,
	}
	if len(c.Tags) > 0 {
		pbTags := make([]*pb.Tag, 0, len(c.Tags))
		for _, t := range c.Tags {
			pbTags = append(pbTags, toTagProto(t))
		}
		proto.Tags = pbTags
	}
	return proto
}

// toTagProto converts a dto.Tag to pb.Tag
func toTagProto(t *dto.Tag) *pb.Tag {
	if t == nil {
		return nil
	}
	return &pb.Tag{
		Id:      strconv.FormatInt(t.ID, 10),
		StoreId: strconv.FormatInt(t.StoreID, 10),
		Name:    t.Name,
	}
}
