# delete every connected account on the test platform
import sp
s,o,_ = sp.call("GET","/v1/accounts",{"limit":100})
for a in o.get("data",[]):
    s2,o2,_ = sp.call("DELETE","/v1/accounts/"+a["id"]); print("deleted",a["id"],s2,o2.get("deleted"))
s,o,_ = sp.call("GET","/v1/accounts",{"limit":100}); print("remaining accounts:",len(o["data"]))
