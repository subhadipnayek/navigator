using System.Globalization;
using System.IO.Compression;
using System.Xml.Linq;

namespace NAVigator.Services;

public static class ExcelTransactionWorkbookParser
{
    public static List<ExcelTransactionDraft> Parse(Stream stream, string fileName)
    {
        if (!fileName.EndsWith(".xlsx", StringComparison.OrdinalIgnoreCase))
            throw new InvalidDataException("Only .xlsx files can be imported by the backend.");

        using var archive = new ZipArchive(stream, ZipArchiveMode.Read, leaveOpen: true);
        var sharedStrings = ReadSharedStrings(archive);
        var dateStyleIds = ReadDateStyleIds(archive);
        var sheetPath = ReadFirstSheetPath(archive);
        var sheetEntry = archive.GetEntry(sheetPath) ?? throw new InvalidDataException("The workbook does not contain a readable worksheet.");

        using var sheetStream = sheetEntry.Open();
        var document = XDocument.Load(sheetStream);
        var rows = document.Descendants(SpreadsheetNs + "row").ToList();
        if (rows.Count == 0) return [];

        var headers = ReadRow(rows[0], sharedStrings, dateStyleIds)
            .Select((value, index) => (Header: NormalizeHeader(value), Index: index))
            .Where(x => x.Header.Length > 0)
            .ToDictionary(x => x.Header, x => x.Index, StringComparer.OrdinalIgnoreCase);

        var drafts = new List<ExcelTransactionDraft>();
        foreach (var row in rows.Skip(1))
        {
            var rowNumber = (int?)row.Attribute("r") ?? drafts.Count + 2;
            var values = ReadRow(row, sharedStrings, dateStyleIds);
            if (values.All(string.IsNullOrWhiteSpace)) continue;

            drafts.Add(new ExcelTransactionDraft(
                rowNumber,
                Get(values, headers, "tradedate", "tradedate"),
                Get(values, headers, "tradetype", "tradetype"),
                Get(values, headers, "price", "nav"),
                Get(values, headers, "quantity", "units"),
                Get(values, headers, "schemecode"),
                Get(values, headers, "isin"),
                Get(values, headers, "schemename"),
                Get(values, headers, "notes")));
        }

        return drafts;
    }

    private static List<string?> ReadRow(XElement row, List<string> sharedStrings, HashSet<int> dateStyleIds)
    {
        var values = new List<string?>();
        foreach (var cell in row.Elements(SpreadsheetNs + "c"))
        {
            var reference = (string?)cell.Attribute("r");
            var columnIndex = reference is null ? values.Count : ColumnIndex(reference);
            while (values.Count <= columnIndex) values.Add(null);
            values[columnIndex] = ReadCell(cell, sharedStrings, dateStyleIds);
        }

        return values;
    }

    private static string? ReadCell(XElement cell, List<string> sharedStrings, HashSet<int> dateStyleIds)
    {
        var type = (string?)cell.Attribute("t");
        if (type == "inlineStr")
            return cell.Descendants(SpreadsheetNs + "t").FirstOrDefault()?.Value.Trim();

        var raw = cell.Element(SpreadsheetNs + "v")?.Value;
        if (string.IsNullOrWhiteSpace(raw)) return null;

        if (type == "s" && int.TryParse(raw, NumberStyles.Integer, CultureInfo.InvariantCulture, out var sharedIndex))
            return sharedIndex >= 0 && sharedIndex < sharedStrings.Count ? sharedStrings[sharedIndex] : null;

        var styleId = int.TryParse((string?)cell.Attribute("s"), NumberStyles.Integer, CultureInfo.InvariantCulture, out var parsedStyle)
            ? parsedStyle
            : -1;
        if (dateStyleIds.Contains(styleId) && double.TryParse(raw, NumberStyles.Float, CultureInfo.InvariantCulture, out var serial))
            return DateTime.FromOADate(serial).ToString("yyyy-MM-dd", CultureInfo.InvariantCulture);

        return raw.Trim();
    }

    private static List<string> ReadSharedStrings(ZipArchive archive)
    {
        var entry = archive.GetEntry("xl/sharedStrings.xml");
        if (entry is null) return [];

        using var stream = entry.Open();
        var document = XDocument.Load(stream);
        return document.Descendants(SpreadsheetNs + "si")
            .Select(si => string.Concat(si.Descendants(SpreadsheetNs + "t").Select(t => t.Value)).Trim())
            .ToList();
    }

    private static HashSet<int> ReadDateStyleIds(ZipArchive archive)
    {
        var result = new HashSet<int>();
        var entry = archive.GetEntry("xl/styles.xml");
        if (entry is null) return result;

        using var stream = entry.Open();
        var document = XDocument.Load(stream);
        var customDateFormatIds = document.Descendants(SpreadsheetNs + "numFmt")
            .Where(e => ((string?)e.Attribute("formatCode") ?? string.Empty).Contains('d', StringComparison.OrdinalIgnoreCase))
            .Select(e => int.TryParse((string?)e.Attribute("numFmtId"), out var id) ? id : -1)
            .Where(id => id >= 0)
            .ToHashSet();

        var cellFormats = document.Descendants(SpreadsheetNs + "cellXfs").Elements(SpreadsheetNs + "xf").ToList();
        for (var index = 0; index < cellFormats.Count; index++)
        {
            if (!int.TryParse((string?)cellFormats[index].Attribute("numFmtId"), out var numFmtId)) continue;
            if ((numFmtId >= 14 && numFmtId <= 22) || customDateFormatIds.Contains(numFmtId))
                result.Add(index);
        }

        return result;
    }

    private static string ReadFirstSheetPath(ZipArchive archive)
    {
        var workbook = archive.GetEntry("xl/workbook.xml") ?? throw new InvalidDataException("The workbook is missing xl/workbook.xml.");
        var relationships = archive.GetEntry("xl/_rels/workbook.xml.rels") ?? throw new InvalidDataException("The workbook is missing worksheet relationships.");

        using var workbookStream = workbook.Open();
        var workbookDocument = XDocument.Load(workbookStream);
        var firstSheet = workbookDocument.Descendants(SpreadsheetNs + "sheet").FirstOrDefault()
            ?? throw new InvalidDataException("The workbook does not contain any sheets.");
        var relationshipId = (string?)firstSheet.Attribute(RelationshipNs + "id")
            ?? throw new InvalidDataException("The first worksheet is missing its relationship id.");

        using var relationshipsStream = relationships.Open();
        var relationshipDocument = XDocument.Load(relationshipsStream);
        var target = relationshipDocument.Root?.Elements(RelPackageNs + "Relationship")
            .FirstOrDefault(e => string.Equals((string?)e.Attribute("Id"), relationshipId, StringComparison.Ordinal))
            ?.Attribute("Target")?.Value;
        if (string.IsNullOrWhiteSpace(target))
            throw new InvalidDataException("The first worksheet target could not be resolved.");

        return target.StartsWith("xl/", StringComparison.OrdinalIgnoreCase)
            ? target
            : $"xl/{target.TrimStart('/')}";
    }

    private static string? Get(List<string?> values, Dictionary<string, int> headers, params string[] names)
    {
        foreach (var name in names)
        {
            if (headers.TryGetValue(NormalizeHeader(name), out var index) && index < values.Count)
                return string.IsNullOrWhiteSpace(values[index]) ? null : values[index]!.Trim();
        }
        return null;
    }

    private static int ColumnIndex(string reference)
    {
        var index = 0;
        foreach (var ch in reference.TakeWhile(char.IsLetter))
            index = (index * 26) + char.ToUpperInvariant(ch) - 'A' + 1;
        return Math.Max(0, index - 1);
    }

    private static string NormalizeHeader(string? value) =>
        new((value ?? string.Empty).Trim().ToLowerInvariant().Where(char.IsLetterOrDigit).ToArray());

    private static readonly XNamespace SpreadsheetNs = "http://schemas.openxmlformats.org/spreadsheetml/2006/main";
    private static readonly XNamespace RelationshipNs = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
    private static readonly XNamespace RelPackageNs = "http://schemas.openxmlformats.org/package/2006/relationships";
}

public record ExcelTransactionDraft(
    int RowNumber,
    string? TradeDate,
    string? TradeType,
    string? Price,
    string? Quantity,
    string? SchemeCode,
    string? Isin,
    string? SchemeName,
    string? Notes);
