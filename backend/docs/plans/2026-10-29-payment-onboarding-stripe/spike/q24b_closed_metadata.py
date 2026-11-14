# Step 9g Q24b: is metadata kept on a closed account (retrieve vs list)?
import uuid, sp, q10_lib
q10_lib.VER = "2026-10-28.endive"
from q10_lib import v2
s, a = v2("POST", "/v2/core/accounts", {"metadata": {"store_id": "q24b-" + uuid.uuid4().hex[:6]}}); aid = a["id"]
print("open retrieve metadata set:", v2("GET", f"/v2/core/accounts/{aid}")[1].get("metadata") is not None)
print("close", v2("POST", f"/v2/core/accounts/{aid}/close", {"applied_configurations": []})[0])
print("closed retrieve metadata:", v2("GET", f"/v2/core/accounts/{aid}")[1].get("metadata"))
