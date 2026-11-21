// §4.9: one colour per site or app, following the name, never its rank (the
// GTK GUI's assign_series and palette).
namespace Kc9k.Core;

public sealed class Series
{
    public static readonly string[] Light = ["#2a78d6", "#eb6834", "#1baf7a", "#eda100", "#e87ba4", "#008300", "#6250d6"];
    public static readonly string[] Dark = ["#3987e5", "#d95926", "#199e70", "#c98500", "#d55181", "#0ca30c", "#9085e9"];
    public const string Other = "#898781";

    readonly Dictionary<string, int> slots = [];

    /// <summary>Palette slots for names: a name keeps the slot it had; a new
    /// one takes a free slot, one no remembered name holds if there is one;
    /// never two shown names in one colour; past seven: Other (null).</summary>
    public Dictionary<string, int?> Assign(IReadOnlyList<string> names)
    {
        var output = new Dictionary<string, int?>();
        var used = new HashSet<int>();
        foreach (var name in names)
            if (slots.TryGetValue(name, out var slot) && !used.Contains(slot))
            {
                output[name] = slot;
                used.Add(slot);
            }
        var remembered = slots.Values.ToHashSet();
        foreach (var name in names)
        {
            if (output.ContainsKey(name)) continue;
            var free = Enumerable.Range(0, Light.Length).Where(s => !used.Contains(s)).ToList();
            if (free.Count == 0)
            {
                output[name] = null;
                continue;
            }
            var pick = free.Where(f => !remembered.Contains(f)).DefaultIfEmpty(free[0]).First();
            output[name] = pick;
            used.Add(pick);
            foreach (var k in slots.Where(kv => kv.Value == pick).Select(kv => kv.Key).ToList()) slots.Remove(k);
            slots[name] = pick;
        }
        return output;
    }

    public static string Color(int? slot, bool dark) => slot is int s ? (dark ? Dark : Light)[s] : Other;
}
