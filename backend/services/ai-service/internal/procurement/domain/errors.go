package domain

import "errors"

// ErrIdempotencyKeyConflict 表示同一個買家把同一個 clientRequestID 用在了「另一個」
// request 上。冪等鍵的語意是「同一個命令重送」，不是「同一個買家的萬用鍵」——換了
// 目標 request 就是不同的命令，此時既不能重播舊結果（呼叫端要的是新 request 的
// 改動），也不能靜默丟棄寫入（使用者說的話會憑空消失）。唯一誠實的回應是報衝突。
var ErrIdempotencyKeyConflict = errors.New("procurement: client request id already used for another request")
