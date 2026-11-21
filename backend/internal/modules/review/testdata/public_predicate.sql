NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM buyer.pub_inactive_buyers() AS ib WHERE ib.buyer_id = r.buyer_id)
