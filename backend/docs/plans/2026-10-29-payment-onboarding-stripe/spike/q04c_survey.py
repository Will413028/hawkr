# Q4: survey of all accounts' current requirements state (post settle) -> charges/payouts mapping
import sp
s,o,_=sp.call("GET","/v1/accounts",{"limit":100})
for a in o["data"]:
    r=a["requirements"]; c=a["controller"]
    print(f"{a['id']} {a['country']} collect={c['requirement_collection']} charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']}\n    due={r['currently_due'][:4]}{'...(+%d)'%(len(r['currently_due'])-4) if len(r['currently_due'])>4 else ''} past_due={len(r['past_due'])} pending={r['pending_verification'][:2]} errors={[(e['code'],e['requirement']) for e in r['errors']]} disabled_reason={r['disabled_reason']}")
