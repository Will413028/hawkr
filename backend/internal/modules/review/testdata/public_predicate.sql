NOT EXISTS (SELECT 1 FROM review.hides AS h WHERE h.review_id = r.id AND h.restored_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM review.inactive_authors AS ia WHERE ia.buyer_id = r.buyer_id)
