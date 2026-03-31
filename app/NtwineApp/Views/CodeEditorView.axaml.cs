using Avalonia.Controls;
using AvaloniaEdit;

namespace NtwineApp.Views;

public partial class CodeEditorView : UserControl
{
    public CodeEditorView()
    {
        InitializeComponent();

        var editor = this.FindControl<TextEditor>("Editor");
        if (editor != null)
        {
            editor.Options.ShowEndOfLine = false;
            editor.Options.ShowSpaces = false;
            editor.Options.ShowTabs = false;
            editor.Options.EnableHyperlinks = false;
            editor.Options.HighlightCurrentLine = true;
        }
    }

    public void SetContent(string text, string filename)
    {
        var editor = this.FindControl<TextEditor>("Editor");
        if (editor != null)
        {
            editor.Text = text;
        }
    }
}
