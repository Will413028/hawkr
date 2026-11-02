# Q2: TW recipient account (application-collected, transfers only), full API KYC with TW test bank account
import sp, time
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"TW","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual","tos_acceptance":{"service_agreement":"recipient"}})
i=a["id"]
ind={"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"+886912345678","dob":{"day":1,"month":1,"year":1901},
 "address":{"line1":"address_full_match","city":"Taipei","postal_code":"100","country":"TW"},"id_number":"A123456789"}
for rn in ["SCBLTWTP"]:
  for acc in ["0001234567"]:
    s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":ind,"external_account":{"object":"bank_account","country":"TW","currency":"twd","routing_number":rn,"account_number":acc},
       "business_profile":{"url":"https://accessible.stripe.com","mcc":"5734"},"tos_acceptance":{"date":int(time.time()),"ip":"8.8.8.8"}})
    print("bank SCBLTWTP/0001234567 + individual + tos ->",s,(o.get("error") or {}).get("message","accepted"))
for n in range(6):
    time.sleep(8); a=sp.call("GET","/v1/accounts/"+i)[1]; r=a["requirements"]
    print(f"t+{(n+1)*8}s charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']} due={r['currently_due']} pend={r['pending_verification']} dis={r['disabled_reason']}")
    if a["payouts_enabled"]: break
