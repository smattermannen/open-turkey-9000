// Phrases (§4.3) and imported files, as the GTK GUI words and reads them.
namespace Kc9k.Core;

/// <summary>What a file to import holds, from `kc9k list read PATH`.</summary>
public sealed class ImportFound
{
    public List<string> Phrases { get; set; } = [];
    public List<string> Entries { get; set; } = [];
    public List<string> Sites { get; set; } = [];
    public List<string> Terms { get; set; } = [];
    public List<string> Skipped { get; set; } = [];

    public static ImportFound Parse(string json) => Kc9kJson.Parse<ImportFound>(json);

    /// <summary>'3 phrases, 2 sites'.</summary>
    public string Describe()
    {
        var parts = new List<string>();
        foreach (var (list, one, many) in new[] { (Phrases, "phrase", "phrases"), (Sites, "site", "sites"),
                     (Terms, "rendered term", "rendered terms"), (Skipped, "line KC9K can't use", "lines KC9K can't use") })
            if (list.Count > 0) parts.Add($"{list.Count} {(list.Count == 1 ? one : many)}");
        return parts.Count > 0 ? string.Join(", ", parts) : "nothing";
    }
}

public static class PhraseText
{
    /// <summary>A phrase as it is typed: "[eat, nice] [hello, grandpa] [today]? :3"
    /// (db.Phrase.String).</summary>
    public static string Of(IEnumerable<(IReadOnlyList<string> Terms, bool Necessary)> sets, int length = 0)
    {
        var parts = new List<string>();
        var necessary = 0;
        foreach (var (terms, needed) in sets)
        {
            if (needed) necessary++;
            parts.Add("[" + string.Join(", ", terms) + "]" + (needed ? "" : "?"));
        }
        var text = string.Join(" ", parts);
        return length != 0 && length != Math.Max(necessary, 2) ? $"{text} :{length}" : text;
    }

    public static string Of(Phrase p) => Of(p.Sets.Select(s => ((IReadOnlyList<string>)s.Terms, s.Necessary)), p.Length);

    /// <summary>A phrase's text for comparing: letter case and spacing don't count.</summary>
    public static string Key(string? text) =>
        string.Join(" ", (text ?? "").ToLowerInvariant().Replace(",", ", ").Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries));

    /// <summary>Terms for one word set from lines (one term each): # comments,
    /// blanks and duplicates dropped; brackets and commas can't be in a term.</summary>
    public static List<string> WordSetTerms(IEnumerable<string> lines)
    {
        var output = new List<string>();
        foreach (var line in lines)
        {
            var term = string.Join(" ", line.Split('#', 2)[0].Replace("[", " ").Replace("]", " ")
                .Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries));
            foreach (var raw in term.Split(','))
            {
                var part = raw.Trim();
                if (part.Length > 0 && part.Trim('*', '-', ' ').Length > 0 &&
                    !output.Any(t => string.Equals(t, part, StringComparison.OrdinalIgnoreCase)))
                    output.Add(part);
            }
        }
        return output;
    }
}
