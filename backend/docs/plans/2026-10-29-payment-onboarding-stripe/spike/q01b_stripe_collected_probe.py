# Q1 follow-up after the application-collected block: does requirement_collection=stripe (Express-style) work, and in which countries?
import sp
created = []
ctrl = {"requirement_collection":"stripe","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"express"}}
for c, caps, tos in [("US",{"card_payments":{"requested":True},"transfers":{"requested":True}},None),
                     ("SG",{"card_payments":{"requested":True},"transfers":{"requested":True}},None),
                     ("JP",{"card_payments":{"requested":True},"transfers":{"requested":True}},None),
                     ("TW",{"card_payments":{"requested":True},"transfers":{"requested":True}},None),
                     ("TW",{"transfers":{"requested":True}},{"service_agreement":"recipient"}),
                     ("SG",{"transfers":{"requested":True}},{"service_agreement":"recipient"})]:
    p = {"country":c,"controller":ctrl,"capabilities":caps}
    if tos: p["tos_acceptance"] = tos
    s, o, _ = sp.call("POST", "/v1/accounts", p)
    label = f"stripe-collected express country={c} caps={list(caps)} tos={tos}"
    if s == 200:
        created.append(o["id"])
        sp.show(label, s, o, ["id","type","country","controller","capabilities","charges_enabled","payouts_enabled"])
    else:
        sp.show(label, s, o)
for a in created:
    s, o, _ = sp.call("DELETE", f"/v1/accounts/{a}")
    print("deleted", a, s, o.get("deleted"))
