# Q4: try to provoke requirements.errors with documented failure test values (address_no_match, id_number 111111111, dob 1900-01-01)
import sp, time
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"card_payments":{"requested":True},"transfers":{"requested":True}},"business_type":"individual"})
i=a["id"]
ind={"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"0000000000","dob":{"day":1,"month":1,"year":1900},
 "address":{"line1":"address_no_match","city":"Schenectady","state":"NY","postal_code":"12345","country":"US"},
 "ssn_last_4":"1111","id_number":"111111111"}
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":ind,"business_profile":{"mcc":"5734","url":"https://accessible.stripe.com"},
  "external_account":{"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789"},
  "tos_acceptance":{"date":int(time.time()),"ip":"8.8.8.8"}})
if s!=200: sp.show("update ERROR",s,o)
for n in range(8):
    time.sleep(6); a=sp.call("GET","/v1/accounts/"+i)[1]; r=a["requirements"]
    print(f"t+{(n+1)*6}s charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']} due={r['currently_due']} pend={r['pending_verification']} errors={[(e['code'],e['requirement']) for e in r['errors']]} dis={r['disabled_reason']}")
    if r["errors"] or (a["charges_enabled"]): break
sp.show("errors", 200, a["requirements"]["errors"])
