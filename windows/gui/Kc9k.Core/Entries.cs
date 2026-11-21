// Typed list entries (§4.3 Add: type, Enter): several at once split at
// spaces, word combinations kept whole, as the GTK GUI's split_entries and
// the extension's kc9kCombination read them.
using System.Text;
using System.Text.RegularExpressions;

namespace Kc9k.Core;

public static class Entries
{
    static readonly Regex ComboGroup = new(@"\(([^()]*)\)(?:\s*:\s*(\d+))?");
    public const int MaxBoxes = 9;

    /// <summary>A word combination's boxes, or null when text isn't one.</summary>
    public static List<(List<string> Terms, int Min)>? ParseCombination(string text)
    {
        var value = text.Trim();
        if (!value.Contains('(') && !value.Contains(')')) return null;
        var boxes = new List<(List<string>, int)>();
        bool Bare(string part)
        {
            var term = part.Trim();
            if (term.Length == 0) return true;
            if (term.IndexOfAny([',', ':', '(', ')']) >= 0 || term.Replace("*", "").Trim().Length == 0) return false;
            boxes.Add(([term], 1));
            return true;
        }
        var last = 0;
        foreach (Match m in ComboGroup.Matches(value))
        {
            if (!Bare(value[last..m.Index])) return null;
            var terms = m.Groups[1].Value.Split(',').Select(t => t.Trim()).ToList();
            var need = m.Groups[2].Success ? int.Parse(m.Groups[2].Value) : 1;
            if (!terms.All(t => t.Replace("*", "").Trim().Length > 0) || need < 1 || need > MaxBoxes) return null;
            boxes.Add((terms, need));
            last = m.Index + m.Length;
        }
        var rest = value[last..];
        if (rest.Contains('(') || rest.Contains(')') || !Bare(rest)) return null;
        if (boxes.Sum(b => b.Item2) < 2 || boxes.Count > MaxBoxes) return null;
        return boxes;
    }

    /// <summary>Splits typed entries at spaces, keeping word combinations whole.</summary>
    public static List<string> Split(string text)
    {
        var parts = new List<string>();
        var current = new StringBuilder();
        var depth = 0;
        foreach (var c in text)
        {
            if (char.IsWhiteSpace(c) && depth == 0)
            {
                if (current.Length > 0) parts.Add(current.ToString());
                current.Clear();
                continue;
            }
            depth = Math.Max(depth + (c == '(' ? 1 : c == ')' ? -1 : 0), 0);
            current.Append(c);
        }
        if (current.Length > 0) parts.Add(current.ToString());

        static bool Lone(string part) =>
            Regex.IsMatch(part, @"^\([^()]*\)(:\d+)?$") && ParseCombination(part) is null;

        var output = new List<string>();
        foreach (var part in parts)
        {
            if (output.Count > 0 && (((part.StartsWith('(') || part.StartsWith(':')) && output[^1].Contains(')'))
                                     || Lone(output[^1]) || (Lone(part) && !output[^1].Contains('('))))
                output[^1] += " " + part;
            else
                output.Add(part);
        }
        return output;
    }
}
