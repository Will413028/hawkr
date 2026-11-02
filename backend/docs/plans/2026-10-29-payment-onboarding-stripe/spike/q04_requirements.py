# Q4 (env DOB_YEAR=1902 gives the dob mismatch case; default 1901 = success): requirements -> charges/payouts at different stages. Stages for the happy path are in outputs/q02_custom_kyc.txt.
# Here: failed identity (document + dob mismatch) on a US application-collected account, then fix it.
import sp, time
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"card_payments":{"requested":True},"transfers":{"requested":True}},"business_type":"individual"})
i=a["id"]
ind={"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"0000000000","dob":{"day":1,"month":1,"year":int(__import__("os").environ.get("DOB_YEAR","1901"))},
 "address":{"line1":"address_full_match","city":"Schenectady","state":"NY","postal_code":"12345","country":"US"},
 "ssn_last_4":"0000","id_number":"000000000","verification":{"document":{"front":"file_identity_document_failure"}}}
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":ind,"business_profile":{"mcc":"5734","url":"https://accessible.stripe.com"},
  "external_account":{"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789"},
  "tos_acceptance":{"date":int(time.time()),"ip":"8.8.8.8"}})
if s!=200: sp.show("update ERROR",s,o)
def snap(label):
    s,a,_=sp.call("GET","/v1/accounts/"+i)
    print(f"=== {label}: charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']}")
    r=a["requirements"]; print(" ",{k:r[k] for k in ("currently_due","past_due","pending_verification","disabled_reason","current_deadline")})
    print("  errors:",[(e['code'],e['requirement']) for e in r['errors']])
    return a
for n in range(10):
    time.sleep(6); a=snap(f"failed document t+{(n+1)*6}s")
    if a["requirements"]["errors"]: break
sp.show("errors detail (full)",200,a["requirements"]["errors"])
# fix with good document
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":{"verification":{"document":{"front":"file_identity_document_success"}}}})
print("resubmit good doc:",s)
for n in range(10):
    time.sleep(6); a=snap(f"after good doc t+{(n+1)*6}s")
    if a["charges_enabled"] and a["payouts_enabled"]: break
