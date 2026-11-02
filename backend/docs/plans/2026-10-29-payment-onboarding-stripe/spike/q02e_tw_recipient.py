# Q2/Q1: TW application-collected recipient account (transfers only): requirements and whether KYC can be completed by API
import sp, time
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"TW","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual","tos_acceptance":{"service_agreement":"recipient"}})
i=a["id"]; print("created TW",s, a.get("default_currency"))
sp.show("[TW] requirements after create",s,sp.state(a)["requirements"],["currently_due","disabled_reason"])
for label,bank in [("TW bank twd 0040000/000123456789",{"object":"bank_account","country":"TW","currency":"twd","routing_number":"0040000","account_number":"000123456789"}),
                   ("TW bank twd 1100000",{"object":"bank_account","country":"TW","currency":"twd","routing_number":"1100000","account_number":"000123456789"})]:
    s,o,_=sp.call("POST","/v1/accounts/"+i,{"external_account":bank})
    print(label,"->",s,(o.get("error") or {}).get("message","accepted"))
    if s==200: break
ind={"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"+886912345678","dob":{"day":1,"month":1,"year":1901},
 "address":{"line1":"address_full_match","city":"Taipei","postal_code":"100","country":"TW"},"id_number":"A123456789"}
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":ind,"business_profile":{"url":"https://accessible.stripe.com","mcc":"5734"},"tos_acceptance":{"date":int(time.time()),"ip":"8.8.8.8"}})
print("submit individual+tos:",s,(o.get("error") or {}).get("message","accepted"))
time.sleep(40)
a=sp.call("GET","/v1/accounts/"+i)[1]; sp.show("[TW] state after 40s",200,sp.state(a))
print("capabilities:",a["capabilities"])
