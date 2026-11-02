# Q8: Files API purpose for identity documents and how it attaches to individual.verification.document
import sp, zlib, struct
def png(w=600,h=400):
    raw=b"".join(b"\x00"+b"\xff\xff\xff"*w for _ in range(h))
    def ch(t,d): c=struct.pack(">I",len(d))+t+d; return c+struct.pack(">I",zlib.crc32(t+d)&0xffffffff)
    return b"\x89PNG\r\n\x1a\n"+ch(b"IHDR",struct.pack(">IIBBBBB",w,h,8,2,0,0,0))+ch(b"IDAT",zlib.compress(raw))+ch(b"IEND",b"")
img=png()
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual"})
i=a["id"]
FH="https://files.stripe.com"
for purpose in ["identity_document","additional_verification","account_requirement","dispute_evidence","bogus_purpose"]:
    s,f,_=sp.call("POST","/v1/files",{"purpose":purpose},files={"file":("front.png",img,"image/png")},host=FH)
    if s==200: print(f"purpose={purpose}: HTTP {s} type={f.get('type')} size={f.get('size')} purpose_echo={f.get('purpose')} expires_at={f.get('expires_at')} keys={sorted(f)}")
    else: print(f"purpose={purpose}: HTTP {s} {f['error'].get('message')}")
    if purpose=="identity_document": fid=f["id"]
# attach
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":{"verification":{"document":{"front":fid}}}})
print("attach identity_document file to individual.verification.document.front:",s, (o.get("error") or {}).get("message"))
if s==200: print("individual.verification.document in response -> keys:", sorted(o["individual"]["verification"]["document"]) if "individual" in o else "(individual not expanded)")
# Person-level / retrieve with individual
s,ind,_=sp.call("GET","/v1/accounts/"+i)
r=sp.call("GET",f"/v1/accounts/{i}/persons")[1]["data"]
print("persons:",len(r),"verification.document keys:",sorted(r[0].get("verification",{}).get("document",{})) if r else None)
# wrong purpose attached
s,f2,_=sp.call("POST","/v1/files",{"purpose":"dispute_evidence"},files={"file":("x.png",img,"image/png")},host=FH)
s,o,_=sp.call("POST","/v1/accounts/"+i,{"individual":{"verification":{"document":{"front":f2["id"]}}}})
print("attach dispute_evidence file as ID document:",s,(o.get("error") or {}).get("message"))
# file retrieval
s,fr,_=sp.call("GET","/v1/files/"+fid); print("retrieve file:",s,{k:fr.get(k) for k in("purpose","type","size","title")})
# file_links not allowed for identity_document?
s,fl,_=sp.call("POST","/v1/file_links",{"file":fid}); print("file_link for identity_document:",s,(fl.get("error") or {}).get("message","created"))
# oversize / wrong type
s,f3,_=sp.call("POST","/v1/files",{"purpose":"identity_document"},files={"file":("x.txt",b"hello","text/plain")},host=FH); print("text/plain upload:",s,(f3.get("error") or {}).get("message","accepted"))
