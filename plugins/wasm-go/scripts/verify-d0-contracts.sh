#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
exec python3 - "$@" <<'PY'
import argparse,hashlib,json,os,pathlib,subprocess,sys,time,uuid

parser=argparse.ArgumentParser(description='Pinned WASM D0 offline contract and emulator checks')
parser.add_argument('--batch',default='A')
parser.add_argument('--output',required=True,type=pathlib.Path)
parser.add_argument('--lock',type=pathlib.Path,default=pathlib.Path('testdata/contracts/contracts.lock.json'))
parser.add_argument('--cases-root',type=pathlib.Path,default=pathlib.Path('testdata/execution'))
parser.add_argument('--run',default='TestD0Identity|TestD0Matcher|TestD0Snapshot|TestD0Callout|TestD0Schedule|TestHostMatch|TestParseOverrideConfig|TestRuleLevelConfigIsolation')
args=parser.parse_args();root=pathlib.Path.cwd();output=args.output.resolve();output.mkdir(parents=True,exist_ok=True)
def sha(raw):return hashlib.sha256(raw).hexdigest()
def command_output(argv):
 result=subprocess.run(argv,cwd=root,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True)
 return result.stdout.strip() if result.returncode==0 else None
report={'report_format_version':'wasm-d0-run-v1','run_id':str(uuid.uuid4()),'run_kind':'offline_fixture','evidence_level':'offline_fixture','subject_layers':['contract_fixture','host_emulator'],'status':'failed','real_gateway_execution':'not_run','product_execution':'not_run','supported':'not_evaluated','IF09_export':'not_run','batch':args.batch,'git':{'commit':command_output(['git','rev-parse','HEAD']),'worktree_changes':command_output(['git','status','--short'])},'inputs':[],'commands':[],'assertions':[],'uncovered':['P1 trusted CallIdentity writer and production selection/Consume/guard','Envoy reentry and real gateway EB cases','GetHttpCallResponseBody read error is a fixture model; no SDK error injection capability','Duplicate/closed-context SDK behavior is probed and is not a production lifecycle proof','Batch B/C and formal IF09 evidence export are not installed']}
environment=dict(os.environ,GOWORK='off',GOPROXY='off',GOSUMDB='off',GOTOOLCHAIN='local',PIP_NO_INDEX='1',PYTHONDONTWRITEBYTECODE='1',D0_CONTRACT_LOCK=str(args.lock.resolve()),D0_EXECUTION_FIXTURE_ROOT=str(args.cases_root.resolve()))
def save():
 (output/'summary.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'status':report['status'],'case_count':report.get('case_count'),'error_code':report.get('error_code'),'summary':str(output/'summary.json')},ensure_ascii=False))
def fail(code,detail,status='failed'):
 report['status']=status;report['error_code']=code;report['detail']=detail;save();raise SystemExit(1)
if '/' in args.run:fail('E_RUN_SCOPE','--run accepts top-level test selection only; subtest filters can omit frozen catalog cases')
def run(name,argv,cwd,allow_existing_provider_path=False):
 start=time.monotonic();result=subprocess.run(argv,cwd=cwd,env=environment,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
 destination=output/(name+'.log');destination.write_bytes(result.stdout)
 report['commands'].append({'name':name,'argv':argv,'cwd':str(cwd),'exit_code':result.returncode,'duration_seconds':round(time.monotonic()-start,3),'output':destination.name,'output_sha256':sha(result.stdout)})
 if result.returncode:
  text=result.stdout.decode(errors='replace');blocked=any(word in text for word in ('E_DEPENDENCY','E_SCOPE_BLOCKED','module lookup disabled','cannot find module','HOST_CAPABILITY_BLOCKED'))
  if allow_existing_provider_path and 'replacement directory /home/cloudyi/CodeWorkspace/higress-plugin/proxy-wasm-go-sdk does not exist' in text:
   report['commands'][-1]['status']='blocked_existing_module_path'
   return None
  code=next((word for word in ('E_DIGEST','E_PATH','E_FILE','E_DEPENDENCY','E_SCOPE_BLOCKED','E_NO_CHECKS') if word in text),'E_COMMAND_FAILED')
  fail(code,name+' failed; see '+destination.name,'blocked' if blocked else 'failed')
 return result.stdout
try:
 lock_raw=args.lock.read_bytes();report['inputs'].append({'path':str(args.lock.resolve()),'sha256':sha(lock_raw),'kind':'contracts_lock'})
 docs={}
 for name,required in [('identity-body.json',13),('matcher-cases.json',11),('callback-schedules.json',15)]:
  path=args.cases_root/name;raw=path.read_bytes();doc=json.loads(raw)
  if len(doc.get('cases',[]))!=required:fail('E_NO_CHECKS',name+' frozen case count differs')
  docs[name]=doc;report['inputs'].append({'path':str(path.resolve()),'sha256':sha(raw),'kind':'fixture'})
 tool_raw=(root/'tools/contractcheck/testdata/tool-lock.json').read_bytes();report['inputs'].append({'path':'tools/contractcheck/testdata/tool-lock.json','sha256':sha(tool_raw),'kind':'tool_lock'})
 tool_paths=['tools/contractcheck/main.go','tools/contractcheck/lock_test.go','tools/contractcheck/schema_test.go','tools/contractcheck/go.mod','internal/executionfixture/loader.go','internal/executionfixture/host_adapter.go','internal/executionfixture/snapshot.go','internal/executionfixture/identity_fixture_test.go','internal/executionfixture/matcher_fixture_test.go','internal/executionfixture/scheduler_test.go','internal/executionfixture/driver_test.go','scripts/verify-d0-contracts.sh','go.mod','go.sum','extensions/ai-proxy/go.mod','extensions/ai-proxy/go.sum']
 report['tool_files']=[{'path':path,'sha256':sha((root/path).read_bytes())} for path in tool_paths]
except (OSError,ValueError) as error:fail('E_FILE','missing or invalid explicit input: '+str(error))
if args.batch!='A':fail('E_SCOPE_BLOCKED','only batch A is installed','blocked')
tool=root/'tools/contractcheck'
run('public-contracts',['go','run','.','verify','--lock',str(args.lock.resolve()),'--batch',args.batch,'--report',str(output/'public-contracts.json')],tool)
public=json.loads((output/'public-contracts.json').read_text())
if public.get('status')!='passed' or public.get('summary',{}).get('executed',0)<=0:fail('E_NO_CHECKS','public contract validator executed no passing checks')
imported=[]
for package in json.loads(lock_raw)['packages']:
 base=root/package['vendor_root'];manifest_raw=(base/'bundle-manifest.json').read_bytes();manifest=json.loads(manifest_raw)
 imported.append({'path':str((base/'bundle-manifest.json').relative_to(root)),'sha256':sha(manifest_raw),'kind':'manifest'})
 for kind in ('entries','fixtures','assets'):
  for item in manifest[kind]:
   path=base/item['path'];imported.append({'path':str(path.relative_to(root)),'sha256':sha(path.read_bytes()),'kind':kind})
import_raw=(json.dumps(imported,ensure_ascii=False,indent=2)+'\n').encode();(output/'imported-files.json').write_bytes(import_raw);report['imported_files']={'path':'imported-files.json','count':len(imported),'sha256':sha(import_raw)}
tool_logs=run('consumer-tests',['go','test','-json','-count=1','./...'],tool)
fixture_logs=run('fixture-tests',['go','test','-json','./internal/executionfixture','./pkg/matcher','-run',args.run,'-count=1'],root)
def passed_events(raw):
 result=[]
 for row in raw.splitlines():
  try:event=json.loads(row)
  except ValueError:continue
  if event.get('Action')=='pass' and event.get('Test'):result.append(event['Test'])
 return result
tool_passed=passed_events(tool_logs);fixture_passed=passed_events(fixture_logs)
required={'TestD0IdentityCorpus','TestD0MatcherCorpus','TestD0ScheduleCorpus','TestD0CalloutExplicitIDForwardAndReverse','TestD0SnapshotCopiesPrivateInputs','TestD0CalloutMissingCapabilityIsBlocked','TestD0CalloutEmulatorCapabilityProbe','TestHostMatch','TestParseOverrideConfig','TestRuleLevelConfigIsolation'}
missing=sorted(required-set(fixture_passed))
if not tool_passed or missing:fail('E_NO_CHECKS','zero/omitted mandatory tests: '+', '.join(missing))
case_test_evidence={}
for filename,parent in [('matcher-cases.json','TestD0MatcherCorpus'),('callback-schedules.json','TestD0ScheduleCorpus')]:
 expected={parent+'/'+case['case_id'] for case in docs[filename]['cases']}
 omitted=sorted(expected-set(fixture_passed))
 if omitted:fail('E_NO_CHECKS','frozen catalog cases did not pass: '+', '.join(omitted))
 case_test_evidence[filename]=sorted(expected)
report['case_test_evidence']=case_test_evidence
provider_root=root/'extensions/ai-proxy'
provider_logs=run('provider-direct',['go','test','-json','./provider','-count=1'],provider_root,allow_existing_provider_path=True)
if provider_logs is None:
 module_raw=(provider_root/'go.mod').read_bytes();sum_raw=(provider_root/'go.sum').read_bytes();module=module_raw.decode()
 sdk='github.com/higress-group/proxy-wasm-go-sdk';version='v0.0.0-20251103120604-77e9cce339d2'
 broken='replace '+sdk+' => /home/cloudyi/CodeWorkspace/higress-plugin/proxy-wasm-go-sdk'
 local='replace github.com/higress-group/wasm-go => ../..'
 if module.count(broken)!=1 or module.count(local)!=1 or sdk+' '+version not in module:fail('E_PROVIDER_ADAPTER','existing module does not match the narrowly allowed offline adapter')
 adapted=module.replace(broken,'').replace(local,'replace github.com/higress-group/wasm-go => '+json.dumps(str(root)))
 sums=[sdk+' '+version+' h1:NY33OrWCJJ+DFiLc+lsBY4Ywor2Ik61ssk6qkGF8Ypo=',sdk+' '+version+'/go.mod h1:tRI2LfMudSkKHhyv1uex3BWzcice2s/l8Ah8axporfA=']
 root_sums=(root/'go.sum').read_text().splitlines()
 if any(line not in root_sums for line in sums):fail('E_PROVIDER_ADAPTER','fixed SDK checksums differ from root module')
 modfile=output/'provider-offline.mod';sumfile=output/'provider-offline.sum';modfile.write_text(adapted)
 child_sums=sum_raw.decode().splitlines();sumfile.write_text('\n'.join(child_sums+[line for line in sums if line not in child_sums])+'\n')
 report['provider_adapter']={'scope':'temporary output only; production module files remain unchanged','sdk_version':version,'sdk_checksums':sums,'original_mod_sha256':sha(module_raw),'original_sum_sha256':sha(sum_raw),'exact_patch':[{'remove':broken},{'replace':local,'with':'replace github.com/higress-group/wasm-go => '+json.dumps(str(root))}],'generated_mod':modfile.name,'generated_mod_sha256':sha(modfile.read_bytes()),'generated_sum':sumfile.name,'generated_sum_sha256':sha(sumfile.read_bytes())}
 report['uncovered'].append('Original provider command is blocked by the existing absolute SDK replace; regression uses the recorded temporary fixed-version -modfile')
 provider_logs=run('provider-offline-adapter',['go','test','-mod=readonly','-modfile',str(modfile),'-json','./provider','-count=1'],provider_root)
if not passed_events(provider_logs):fail('E_NO_CHECKS','provider regression selected zero tests')
for filename,test in [('identity-body.json','TestD0IdentityCorpus'),('matcher-cases.json','TestD0MatcherCorpus'),('callback-schedules.json','TestD0ScheduleCorpus')]:
 for case in docs[filename]['cases']:
  proof=test if filename=='identity-body.json' else test+'/'+case['case_id']
  report['assertions'].append({'case_id':case['case_id'],'status':'matched','expected':case['expected'],'proof_test':proof,'evidence_level':'offline_fixture','subject_layer':'host_emulator' if filename=='matcher-cases.json' else 'contract_fixture','product_execution':'not_run'})
report['case_count']={'public_contract':public['summary']['executed'],'identity_body':13,'matcher':11,'callback_schedule':15,'consumer_test_events':len(tool_passed),'fixture_test_events':len(fixture_passed),'provider_test_events':len(passed_events(provider_logs))}
report['status']='passed';save()
PY
