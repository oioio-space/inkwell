// Fixed actor and timestamp make the 50,000-operation document reproducible.
const fs = require("node:fs");
const A = require("../../frontend/node_modules/@automerge/automerge");
let doc = A.init({actor: "0123456789abcdef0123456789abcdef"});
doc = A.change(doc, {time: 0}, d => { d.text = ""; });
doc = A.change(doc, {time: 0}, d => { A.splice(d, ["text"], 0, 0, "a".repeat(50000)); });
fs.mkdirSync("testdata/perf/generated/automerge", {recursive:true});
fs.writeFileSync("testdata/perf/generated/automerge/50000.automerge", A.save(doc));
fs.writeFileSync("testdata/perf/generated/automerge/operations.json", JSON.stringify({
  textCharacters:50000, textInsertOperations:50000, metadataOperations:1, changes:A.getAllChanges(doc).length
}));
