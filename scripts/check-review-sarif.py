#!/usr/bin/env python3
"""Validate a local SARIF output against the supplied official OASIS schema.

Usage: python3 scripts/check-review-sarif.py SCHEMA OUTPUT
Requires the development-only Python jsonschema package. Does not fetch data.
"""
import json
import sys


def main():
    try:
        import jsonschema
        if len(sys.argv) != 3:
            raise ValueError()
        with open(sys.argv[1], encoding="utf-8") as source:
            schema = json.load(source)
        if schema.get("id") != "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json":
            raise ValueError()
        # The official schema uses only local fragments. Never resolve a
        # network reference from a caller-supplied schema with a copied id.
        pending = [schema]
        while pending:
            node = pending.pop()
            if isinstance(node, dict):
                if "$ref" in node and not node["$ref"].startswith("#"):
                    raise ValueError()
                pending.extend(node.values())
            elif isinstance(node, list):
                pending.extend(node)
        with open(sys.argv[2], encoding="utf-8") as source:
            output = json.load(source)
        jsonschema.Draft4Validator.check_schema(schema)
        jsonschema.Draft4Validator(schema).validate(output)
    except Exception:
        print("SARIF_SCHEMA_CHECK_FAILED: details omitted")
        return 1
    print("SARIF_SCHEMA_CHECK_PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
