import sys, time, json, sp
from q10_lib import v2
log=sys.argv[1]
s,a=v2("POST","/v2/core/accounts",{"contact_email":"spike@example.com","display_name":"Thin","identity":{"country":"us","entity_type":"individual"},
  "configuration":{"recipient":{"capabilities":{"stripe_balance":{"stripe_transfers":{"requested":True}}}}},
  "defaults":{"responsibilities":{"fees_collector":"application","losses_collector":"application"}},"dashboard":"none"})
print("create",s); aid=a["id"]
time.sleep(3)
print("update",v2("POST",f"/v2/core/accounts/{aid}",{"identity":{"individual":{"given_name":"Jenny","surname":"Rosen"}}})[0])
time.sleep(25)
recs=[json.loads(l) for l in open(log)]
for r in recs: print(json.dumps(r))
for r in recs[:2]:
    s,e=v2("GET",f"/v2/core/events/{r['event']}",ctx=r.get("context"))
    sp.show("retrieve "+r["type"],s,e,["object","type","related_object","data","changes","context","reason"])
print("delete",sp.call("DELETE","/v1/accounts/"+aid)[0])
