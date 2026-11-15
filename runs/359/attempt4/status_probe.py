#!/usr/bin/env python3
"""#359 P2 probe: how long does a status last? Gives the leader a move and uses it on a wild monster.

Usage: python status_probe.py ROM OUT.jsonl MOVE_ID [--frames N] [--uses K]
ROM is a scratch-named copy of pmdb.nds outside the repo; OUT.jsonl is outside the repo too.
Inputs: boot_to_dungeon.txt, then combat_inputs.txt from the first dungeon frame (A attacks).
FORCED, not natural (two pokes, both labelled in the output):
  1. move slot 0 of team[0] (info+0xF4: flags +0, id u16 +2, PP +4) gets MOVE_ID and PP 99, at the leader's first attack;
  2. while a wild monster is within 1 tile, the next K Regular Attacks (action 0x32 at info+0x42, facing info+0x44 turned to it, seen at
     the entry of ExecuteEntityDungeonAction 0218453C) become action 0x14 (move slot in info+0x4C = 0),
     the game's own move-menu path. Later Regular Attacks become action 3 (a no-op that passes the turn),
     so the target is not killed or hit again.
Trace: one JSON line when the bytes info+0x99..0xEE or the HP of any entity change, sampled at each
RunFractionalTurn and after each TickStatusAndHealthRegen call from 02140BA0 (the turn counter "t").
Prints the SHA-256 of the trace file and, per entity, the first status bytes seen."""
import argparse, hashlib, json, os, sys
os.environ.setdefault("SDL_VIDEODRIVER", "dummy"); os.environ.setdefault("SDL_AUDIODRIVER", "dummy")
V = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, V)
from desmume.emulator import DeSmuME
from retail_trace import parse_inputs, Ram, G_DUNGEON, TRIGGER, TEAM, WILD

p = argparse.ArgumentParser()
p.add_argument("rom"); p.add_argument("out"); p.add_argument("move", type=lambda s: int(s, 0))
p.add_argument("--frames", type=int, default=6000); p.add_argument("--uses", type=int, default=1)
p.add_argument("--skip", type=int, default=0, help="Regular Attacks with a monster adjacent to let pass first (changes the RNG state at the use)")
a = p.parse_args()
emu = DeSmuME(); emu.open(a.rom); ram = Ram(emu); mem = emu.memory; R = mem.register_arm9
boot = parse_inputs(V + "/boot_to_dungeon.txt"); turns = parse_inputs(V + "/combat_inputs.txt")
DIRS = {(0, 1): 0, (1, 1): 1, (1, 0): 2, (1, -1): 3, (0, -1): 4, (-1, -1): 5, (-1, 0): 6, (-1, 1): 7}
EXEC, AFTER_TICK, LO, HI = 0x0218453C, 0x02140BA4, 0x99, 0xEE
st = {"acc_pending": False, "acc_lr": set(), "seen": 0, "dfr": None, "poked": False, "uses": 0, "t": 0, "last": {}}
rows = []; fr = [0]

def ents(d):
    out = []
    for base, n in ((TEAM, 4), (WILD, 16)):
        for i in range(n):
            e = ram.u32(d + base + 4 * i)
            if e and ram.u32(e): out.append((e, ram.u32(e + 0x70)))
    return out

def log(*x): rows.append([fr[0], st["t"], *x])

def sample(*_):
    d = ram.u32(G_DUNGEON)
    if st["dfr"] is None or not d: return
    for e, info in ents(d):
        cur = ram.bytes(info + LO, HI - LO + 1).hex() + ":%d" % ram.s16(info + 0xE)
        if st["last"].get(e) != cur:
            st["last"][e] = cur
            log("chg", hex(e), "sp", ram.s16(info + 2), "info6", ram.u8(info + 6), cur)

def tick(*_):
    if st["dfr"] is not None: st["t"] += 1; sample()

def on_exec(*_):
    d = ram.u32(G_DUNGEON)
    if st["dfr"] is None or not d: return
    e = int(R.r0) & 0xFFFFFFFF
    if e != ram.u32(d + TEAM): return
    info = ram.u32(e + 0x70)
    if ram.u8(info + 0x42) != 0x32: return
    if not st["poked"]:  # the slots are filled after the dungeon pointer is set, so poke at the first leader attack
        slot = info + 0xF4
        log("FORCED_poke_slot0", "was", ram.bytes(slot, 6).hex())
        mem.write_byte(slot + 2, a.move & 0xFF); mem.write_byte(slot + 3, a.move >> 8); mem.write_byte(slot + 4, 99)
        st["poked"] = True
    x, y = ram.s16(e + 4), ram.s16(e + 6)
    near = [w for w, wi in ents(d) if ram.u8(wi + 6) == 1 and max(abs(ram.s16(w + 4) - x), abs(ram.s16(w + 6) - y)) == 1]
    if near and st["seen"] < a.skip:
        st["seen"] += 1
    elif near and st["uses"] < a.uses:
        st["uses"] += 1; mem.write_byte(info + 0x42, 0x14); mem.write_byte(info + 0x4C, 0)
        dx, dy = ram.s16(near[0] + 4) - x, ram.s16(near[0] + 6) - y  # 0 down, then clockwise (docs/re/ai.md)
        mem.write_byte(info + 0x44, DIRS[(dx, dy)])
        log("FORCED_use_move", a.move, "target", hex(near[0]), "face", ram.u8(info + 0x44), "pp", ram.u8(info + 0xF4 + 4))
    elif st["uses"] >= a.uses:
        mem.write_byte(info + 0x42, 3)

# entry of the move dispatcher, the accuracy check and each status setter (docs/re/status.md): proves the effect ran
WATCH = (0x0215BE14, 0x02156824, 0x0215A928, 0x021885A0, 0x0218A710, 0x0218B2D0, 0x021887E0, 0x0218D778, 0x0218D5F0,
         0x0218E428, 0x0218DAE4, 0x021882D0)
def watch(addr):
    def h(*_):
        if st["dfr"] is not None and st["uses"]: log("call", hex(addr), "r0", hex(int(R.r0) & 0xFFFFFFFF), "r1", hex(int(R.r1) & 0xFFFFFFFF))
    mem.register_exec(addr, h)
for w in WATCH: watch(w)
ACC = 0x0215A928  # AccuracyCheck(user, target, move, slot): true = hit; log the return value of the leader's checks
def on_acc(*_):
    d = ram.u32(G_DUNGEON)
    if st["dfr"] is None or not d or not st["uses"] or int(R.r0) & 0xFFFFFFFF != ram.u32(d + TEAM): return
    lr = int(R.r14) & 0xFFFFFFFF
    mv = int(R.r2) & 0xFFFFFFFF
    log("acc_move", hex(mv), ram.bytes(mv, 8).hex(), "slot_arg", int(R.r3) & 0xFFFFFFFF)
    st["acc_pending"] = True
    if lr not in st["acc_lr"]:
        st["acc_lr"].add(lr); mem.register_exec(lr, on_ret)
def on_ret(*_):
    if st["acc_pending"]: st["acc_pending"] = False; log("acc_ret", int(R.r0) & 0xFF)
mem.register_exec(ACC, on_acc)
mem.register_exec(EXEC, on_exec); mem.register_exec(TRIGGER, sample); mem.register_exec(AFTER_TICK, tick)
for f in range(a.frames):
    fr[0] = f
    if st["dfr"] is None:
        if f in boot: emu.input.keypad_update(boot[f])
    else:
        t = f - st["dfr"]
        if t in turns: emu.input.keypad_update(turns[t])
    emu.cycle(with_joystick=False)
    if st["dfr"] is None and f % 10 == 0 and ram.u32(G_DUNGEON) != 0: st["dfr"] = f
body = "\n".join(json.dumps(r) for r in rows) + "\n"
open(a.out, "w").write(body)
print("dungeon_frame", st["dfr"], "rows", len(rows), "uses", st["uses"], "ticks", st["t"])
print("SHA256", hashlib.sha256(body.encode()).hexdigest())
emu.destroy()
