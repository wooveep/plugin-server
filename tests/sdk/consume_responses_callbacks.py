from pathlib import Path
import argparse,hashlib,json
import httpx2 as httpx
import openai
from openai import OpenAI

parser=argparse.ArgumentParser(description="Consume actual managed callback SSE using the official SDK without network")
parser.add_argument("evidence_dir",type=Path)
root=parser.parse_args().evidence_dir.resolve()
results=[]
for name,expected_status in [('separate-stop','completed'),('separate-length','incomplete'),('tools-stream','completed')]:
    path=root/'callbacks'/(name+'.sse');raw=path.read_bytes()
    wire=[json.loads(line[6:]) for line in raw.decode().splitlines() if line.startswith('data: ')]
    assert [event['sequence_number'] for event in wire]==list(range(len(wire)))
    assert wire[0]['type']=='response.created'
    requests=[]
    def serve(request):
        requests.append(request.method)
        return httpx.Response(200,headers={'content-type':'text/event-stream'},content=raw)
    client=OpenAI(api_key='controlled-local-sdk-fixture',base_url='http://in-process-sdk-fixture.invalid/v1',max_retries=0,http_client=httpx.Client(transport=httpx.MockTransport(serve)))
    added={};done={};snapshots={};events=[]
    with client.responses.stream(model='controlled-fixture',input='controlled-fixture') as stream:
        for event in stream:
            events.append(event)
            if event.type=='response.output_item.added':
                assert event.output_index not in added
                added[event.output_index]=event.item.id
            elif event.type=='response.output_item.done':
                assert added[event.output_index]==event.item.id
                done[event.output_index]=event.item.model_dump(exclude_none=True)
            elif event.type in ('response.output_text.delta','response.function_call_arguments.delta'):
                snapshots[event.item_id]=event.snapshot
        terminal=events[-1]
        assert terminal.type=='response.'+expected_status
        response=stream.get_final_response() if expected_status=='completed' else terminal.response
    assert response.status==expected_status
    assert requests==['POST']
    assert len(added)==len(done)==len(response.output)
    assert [done[index] for index in sorted(done)]==[item.model_dump(exclude_none=True) for item in response.output]
    assert response.usage.input_tokens==9007199254740997 and response.usage.output_tokens==5
    if name=='tools-stream':
        assert [item.type for item in response.output]==['function_call','message','function_call']
        first,text,second=response.output
        assert first.name=='first_tool' and json.loads(first.arguments)['value']==9007199254740993
        assert text.content[0].text=='checking' and second.name=='second_tool'
        assert snapshots[first.id]==first.arguments and snapshots[text.id]=='checking' and snapshots[second.id]==second.arguments
    elif expected_status=='incomplete':
        assert response.incomplete_details.reason=='max_output_tokens'
    client.close()
    results.append({'case':name,'result':'PASS','sdk_events':len(events),'wire_events':len(wire),'status':response.status,'output_items':len(response.output),'source_sse_sha256':hashlib.sha256(raw).hexdigest(),'in_process_mock_requests':len(requests),'network_or_supplier_calls':0})
proof={'status':'PASS','openai_sdk_version':openai.__version__,'source':'Actual managed sole-sender host callback SSE after supplier/client conversion and exact usage restoration','transport':'httpx2.MockTransport (SDK dependency); no network or supplier calls','cases':results}
(root/'sdk-consumer.json').write_text(json.dumps(proof,indent=2)+'\n')
print(json.dumps(proof))
