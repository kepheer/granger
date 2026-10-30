#!/usr/bin/python3
import hashlib
import json
import pathlib
import re
import urllib.error
import urllib.request

SOURCE = 'https://raw.githubusercontent.com/runetfreedom/russia-domains-list/main/ru-available-only-inside'
INCLUDES = 'https://raw.githubusercontent.com/v2fly/domain-list-community/master/data/'
BASE = 'http://127.0.0.1:1984/api/'


def fetch(url):
    with urllib.request.urlopen(url, timeout=30) as response:
        body = response.read(1024 * 1024 + 1)
    if len(body) > 1024 * 1024:
        raise ValueError('Domain source is too large')
    return body


def domains(read=fetch):
    result = set()
    visited = set()
    digests = {}

    def parse(url, depth=0):
        if url in visited:
            return
        if depth > 8 or len(visited) >= 64:
            raise ValueError('Too many included domain lists')
        visited.add(url)
        body = read(url)
        digests[url] = hashlib.sha256(body).hexdigest()
        for line in body.decode('utf-8').splitlines():
            value = line.split('#', 1)[0].strip()
            if value == '':
                continue
            value = value.split()[0]
            if value.startswith('include:'):
                name = value.removeprefix('include:')
                if re.fullmatch(r'[a-z0-9][a-z0-9_-]*', name) is None:
                    raise ValueError('Invalid included list name')
                parse(INCLUDES + name, depth + 1)
                continue
            value = value.removeprefix('domain:').removeprefix('*.').lower().strip('.')
            if re.fullmatch(r'[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?', value) is None or '.' not in value or len(value) > 253:
                raise ValueError('Unsupported domain rule: ' + value)
            result.add(value)
    parse(SOURCE)
    if len(result) < 20 or len(result) > 20000:
        raise ValueError('Unexpected domain list size')
    return sorted(result), digests


def api(path, payload=None):
    token = pathlib.Path('/etc/granger/secrets/admin-token').read_text().strip()
    headers = {'Authorization': 'Bearer ' + token, 'X-Granger-Request': '1', 'Content-Type': 'application/json'}
    body = None if payload is None else json.dumps(payload).encode()
    with urllib.request.urlopen(urllib.request.Request(BASE + path, data=body, headers=headers), timeout=120) as response:
        data = json.load(response)
    if data.get('ok') is not True:
        raise RuntimeError(data.get('error', 'Routing update failed'))
    return data


def main():
    values, digests = domains()
    for attempt in range(3):
        graph = api('routing/graph')['data']
        rule = next((node for node in graph['nodes'] if node['id'] == 'rule:ru-direct'), None)
        if rule is None:
            raise RuntimeError('ru-direct rule is missing')
        changed = rule['data'].get('domains', []) != values or rule['data'].get('cidrs', []) != [] or rule['data'].get('domain_match') != 'sni'
        if changed:
            rule['data']['domains'] = values
            rule['data']['cidrs'] = []
            rule['data']['domain_match'] = 'sni'
            try:
                api('routing/graph', graph)
            except urllib.error.HTTPError as error:
                if error.code == 409 and attempt < 2:
                    continue
                raise
        status = {'source': SOURCE, 'domains': len(values), 'ipv4_addresses': 0, 'matcher': 'sni', 'sources_sha256': digests, 'changed': changed}
        target = pathlib.Path('/var/lib/granger/ru-list-status.json')
        temporary = target.with_suffix('.tmp')
        temporary.write_text(json.dumps(status, indent=2))
        temporary.chmod(0o600)
        temporary.replace(target)
        print(json.dumps(status))
        return


if __name__ == '__main__':
    main()
