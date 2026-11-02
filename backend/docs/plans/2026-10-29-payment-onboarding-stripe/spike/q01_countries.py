# Q1: which connected-account countries work under this US platform
import sp, json
created = []
def try_(label, params, idem=None):
    s, o, _ = sp.call("POST", "/v1/accounts", params, idem=idem)
    if s == 200:
        created.append(o["id"])
        sp.show(label, s, o, ["id","type","country","default_currency","controller","capabilities","business_type","tos_acceptance"])
    else:
        sp.show(label, s, o)
    return o
ctrl = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
caps = {"card_payments":{"requested":True},"transfers":{"requested":True}}
for c in ["US","TW","SG","JP"]:
    try_(f"controller/application country={c} full agreement", {"country":c,"controller":ctrl,"capabilities":caps,"business_type":"individual"})
for c in ["TW","SG","JP"]:
    try_(f"legacy type=custom country={c}", {"type":"custom","country":c,"capabilities":caps,"business_type":"individual"})
# cross-border recipient service agreement (docs: required for non-US-country accounts under US platform)
for c in ["TW","SG","JP"]:
    try_(f"controller/application country={c} tos_acceptance.service_agreement=recipient (transfers only)",
         {"country":c,"controller":ctrl,"capabilities":{"transfers":{"requested":True}},"business_type":"individual",
          "tos_acceptance":{"service_agreement":"recipient"}})
for c in ["TW"]:
    try_(f"Express-like controller country={c} requirement_collection=stripe",
         {"country":c,"controller":{"requirement_collection":"stripe","fees":{"payer":"account"},"losses":{"payments":"stripe"},"stripe_dashboard":{"type":"express"}},
          "capabilities":{"transfers":{"requested":True}},"tos_acceptance":{"service_agreement":"recipient"}})
print("--- created ids (for cleanup):", " ".join(created))
open("/tmp/spike_q01_ids.txt","w").write("\n".join(created))
