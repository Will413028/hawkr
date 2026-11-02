# Q2: Custom-equivalent (controller.requirement_collection=application): KYC via API to charges_enabled + payouts_enabled
import sp, time
ctrl = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
caps = {"card_payments":{"requested":True},"transfers":{"requested":True}}
def st(title, a): sp.show(title, 200, sp.state(a))
def get(i): return sp.call("GET","/v1/accounts/"+i)[1]
def upd(i, d, label):
    s,o,_ = sp.call("POST","/v1/accounts/"+i, d)
    if s!=200: sp.show(label+" ERROR", s, o); return get(i)
    return o
made=[]
def run(country, cur, bank, individual, profile):
    s,a,_ = sp.call("POST","/v1/accounts",{"country":country,"controller":ctrl,"capabilities":caps,"business_type":"individual"})
    sp.show(f"[{country}] create", s, {"id":a.get("id"),"controller":a.get("controller"),"capabilities":a.get("capabilities"),"charges_enabled":a.get("charges_enabled"),"payouts_enabled":a.get("payouts_enabled")})
    i=a["id"]; made.append(i)
    st(f"[{country}] after create", get(i))
    a=upd(i,{"individual":individual,"business_profile":profile},"individual+profile"); st(f"[{country}] after individual+business_profile", a)
    a=upd(i,{"external_account":bank},"external_account"); st(f"[{country}] after external_account", a)
    now=int(time.time())
    a=upd(i,{"tos_acceptance":{"date":now,"ip":"8.8.8.8"}},"tos"); st(f"[{country}] after tos_acceptance", a)
    for n in range(4):
        time.sleep(3); a=get(i)
        print(f"[{country}] poll {n}: charges_enabled={a['charges_enabled']} payouts_enabled={a['payouts_enabled']} caps={a['capabilities']}")
    st(f"[{country}] final", a)
    return i
doc={"front":"file_identity_document_success"}
us=run("US","usd",
  {"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789"},
  {"first_name":"Jenny","last_name":"Rosen","email":"jenny.rosen@example.com","phone":"0000000000","dob":{"day":1,"month":1,"year":1901},
   "address":{"line1":"address_full_match","city":"Schenectady","state":"NY","postal_code":"12345","country":"US"},
   "ssn_last_4":"0000","id_number":"000000000","verification":{"document":doc}},
  {"mcc":"5734","url":"https://accessible.stripe.com","product_description":"spike"})
print("--- done US", us)
