# Q2 (non-US): SG individual, application-collected. Second attempt: valid phone, nationality, full_name_aliases.
import sp, time, os
ctrl = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"SG","controller":ctrl,"capabilities":{"card_payments":{"requested":True},"transfers":{"requested":True}},"business_type":"individual"})
i=a["id"]; print("created",i, s)
sp.show("[SG] requirements after create",s,sp.state(a)["requirements"],["currently_due"])
ind={"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"+6581234567","dob":{"day":1,"month":1,"year":1901},
 "nationality":"SG","full_name_aliases":[""],
 "address":{"line1":"address_full_match","city":"Singapore","postal_code":"546080","country":"SG"},
 "id_number":"S0000001I","verification":{"document":{"front":"file_identity_document_success"}}}
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":ind,"business_profile":{"mcc":"5734","url":"https://accessible.stripe.com","product_description":"spike"},
  "external_account":{"object":"bank_account","country":"SG","currency":"sgd","routing_number":"1100-000","account_number":"000123456"},
  "tos_acceptance":{"date":int(time.time()),"ip":"8.8.8.8"}})
if s!=200: sp.show("[SG] update ERROR",s,o)
s,a,_=sp.call("GET","/v1/accounts/"+i); sp.show("[SG] after submit",s,sp.state(a))
for n in range(9):
    time.sleep(10); s,a,_=sp.call("GET","/v1/accounts/"+i); r=a["requirements"]
    print(f"t+{(n+1)*10}s charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']} cd={r['currently_due']} pend={r['pending_verification']} err={[e['code'] for e in r['errors']]}")
    if a["charges_enabled"] and a["payouts_enabled"]: break
sp.show("[SG] final",s,sp.state(a))
