# Windows Pass-the-Hash scenario

A self-contained demo chain: on a Windows target it dumps the SAM hive, extracts an
NTLM hash + username, and pivots via Pass-the-Hash.

```
windows-pth-chain.yaml   The attack chain (run against the lab's win_target)
fixtures/
  sam.hive               Captured SAM registry hive — offline sample for the
                         hash-parsing step. Lab/target artifact only.
  pivot-marker.txt       Proof-of-pivot marker written on success.
ROUTE_PATCH.md           Note: the Windows implant route the main API needs for
                         on-demand Windows beacon delivery (not yet in main).
```

> ⚠️ `fixtures/sam.hive` contains lab-only credential material. It exists solely as a
> deterministic fixture for developing/testing the SAM-parsing step offline. Do not
> reuse outside this isolated range.
