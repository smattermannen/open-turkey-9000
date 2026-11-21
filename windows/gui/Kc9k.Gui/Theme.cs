// The design layer (R-GUI-3, the GTK GUI's APP_CSS): cards, pills, text
// styles, as theme-aware styles. They are written as a XAML string read at
// run time, so {ThemeResource} keeps them right in light, dark and high
// contrast and when the theme changes while the window is open.
using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Markup;
using Microsoft.UI.Xaml.Media;
using Windows.UI;

namespace Kc9k.Gui;

public static class Theme
{
    const string Xaml = """
<ResourceDictionary
    xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation"
    xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml">
  <ResourceDictionary.ThemeDictionaries>
    <ResourceDictionary x:Key="Light">
      <SolidColorBrush x:Key="Kc9kPillOnBackground" Color="#DFF6DD"/>
      <SolidColorBrush x:Key="Kc9kPillOnForeground" Color="#0E700E"/>
      <SolidColorBrush x:Key="Kc9kPillLockedBackground" Color="#FDE7E9"/>
      <SolidColorBrush x:Key="Kc9kPillLockedForeground" Color="#A4262C"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedBackground" Color="#E5F1FB"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedForeground" Color="#005A9E"/>
      <SolidColorBrush x:Key="Kc9kPillOffBackground" Color="#F0F0F0"/>
      <SolidColorBrush x:Key="Kc9kPillOffForeground" Color="#5C5C5C"/>
      <SolidColorBrush x:Key="Kc9kPillWarnBackground" Color="#FFF4CE"/>
      <SolidColorBrush x:Key="Kc9kPillWarnForeground" Color="#8A5300"/>
      <SolidColorBrush x:Key="Kc9kHeroBackground" Color="#EAF3FC"/>
      <SolidColorBrush x:Key="Kc9kErrorForeground" Color="#C42B1C"/>
    </ResourceDictionary>
    <ResourceDictionary x:Key="Dark">
      <SolidColorBrush x:Key="Kc9kPillOnBackground" Color="#1E3A1E"/>
      <SolidColorBrush x:Key="Kc9kPillOnForeground" Color="#9AE29A"/>
      <SolidColorBrush x:Key="Kc9kPillLockedBackground" Color="#442726"/>
      <SolidColorBrush x:Key="Kc9kPillLockedForeground" Color="#FFB3B8"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedBackground" Color="#1C3249"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedForeground" Color="#9CCBF5"/>
      <SolidColorBrush x:Key="Kc9kPillOffBackground" Color="#333333"/>
      <SolidColorBrush x:Key="Kc9kPillOffForeground" Color="#C8C8C8"/>
      <SolidColorBrush x:Key="Kc9kPillWarnBackground" Color="#433519"/>
      <SolidColorBrush x:Key="Kc9kPillWarnForeground" Color="#FCE100"/>
      <SolidColorBrush x:Key="Kc9kHeroBackground" Color="#1F2B38"/>
      <SolidColorBrush x:Key="Kc9kErrorForeground" Color="#FF99A4"/>
    </ResourceDictionary>
    <ResourceDictionary x:Key="HighContrast">
      <SolidColorBrush x:Key="Kc9kPillOnBackground" Color="{ThemeResource SystemColorWindowColor}"/>
      <SolidColorBrush x:Key="Kc9kPillOnForeground" Color="{ThemeResource SystemColorWindowTextColor}"/>
      <SolidColorBrush x:Key="Kc9kPillLockedBackground" Color="{ThemeResource SystemColorHighlightColor}"/>
      <SolidColorBrush x:Key="Kc9kPillLockedForeground" Color="{ThemeResource SystemColorHighlightTextColor}"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedBackground" Color="{ThemeResource SystemColorWindowColor}"/>
      <SolidColorBrush x:Key="Kc9kPillLiftedForeground" Color="{ThemeResource SystemColorHotlightColor}"/>
      <SolidColorBrush x:Key="Kc9kPillOffBackground" Color="{ThemeResource SystemColorWindowColor}"/>
      <SolidColorBrush x:Key="Kc9kPillOffForeground" Color="{ThemeResource SystemColorGrayTextColor}"/>
      <SolidColorBrush x:Key="Kc9kPillWarnBackground" Color="{ThemeResource SystemColorHighlightColor}"/>
      <SolidColorBrush x:Key="Kc9kPillWarnForeground" Color="{ThemeResource SystemColorHighlightTextColor}"/>
      <SolidColorBrush x:Key="Kc9kHeroBackground" Color="{ThemeResource SystemColorWindowColor}"/>
      <SolidColorBrush x:Key="Kc9kErrorForeground" Color="{ThemeResource SystemColorHighlightColor}"/>
    </ResourceDictionary>
  </ResourceDictionary.ThemeDictionaries>

  <Style x:Key="Kc9kCard" TargetType="Border">
    <Setter Property="Background" Value="{ThemeResource CardBackgroundFillColorDefaultBrush}"/>
    <Setter Property="BorderBrush" Value="{ThemeResource CardStrokeColorDefaultBrush}"/>
    <Setter Property="BorderThickness" Value="1"/>
    <Setter Property="CornerRadius" Value="8"/>
    <Setter Property="Padding" Value="16,14,16,14"/>
  </Style>
  <Style x:Key="Kc9kHero" TargetType="Border">
    <Setter Property="Background" Value="{ThemeResource Kc9kHeroBackground}"/>
    <Setter Property="BorderBrush" Value="{ThemeResource CardStrokeColorDefaultBrush}"/>
    <Setter Property="BorderThickness" Value="1"/>
    <Setter Property="CornerRadius" Value="8"/>
    <Setter Property="Padding" Value="20,18,20,18"/>
  </Style>
  <Style x:Key="Kc9kDim" TargetType="TextBlock" BasedOn="{StaticResource BodyTextBlockStyle}">
    <Setter Property="Foreground" Value="{ThemeResource TextFillColorSecondaryBrush}"/>
    <Setter Property="TextWrapping" Value="Wrap"/>
  </Style>
  <Style x:Key="Kc9kNote" TargetType="TextBlock" BasedOn="{StaticResource CaptionTextBlockStyle}">
    <Setter Property="Foreground" Value="{ThemeResource TextFillColorSecondaryBrush}"/>
    <Setter Property="TextWrapping" Value="Wrap"/>
  </Style>
  <Style x:Key="Kc9kProblem" TargetType="TextBlock" BasedOn="{StaticResource CaptionTextBlockStyle}">
    <Setter Property="Foreground" Value="{ThemeResource Kc9kErrorForeground}"/>
    <Setter Property="TextWrapping" Value="Wrap"/>
  </Style>
  <Style x:Key="Kc9kPageTitle" TargetType="TextBlock" BasedOn="{StaticResource TitleTextBlockStyle}"/>
  <Style x:Key="Kc9kSectionTitle" TargetType="TextBlock" BasedOn="{StaticResource BodyStrongTextBlockStyle}">
    <Setter Property="FontSize" Value="16"/>
  </Style>
</ResourceDictionary>
""";

    public static void AddResources(ResourceDictionary resources)
    {
        resources.MergedDictionaries.Add((ResourceDictionary)XamlReader.Load(Xaml));
    }

    public static Style Style(string key) => (Style)Application.Current.Resources[key];

    public static Brush Brush(string key) => (Brush)Application.Current.Resources[key];

    /// <summary>The theme for the window's content from the preference.</summary>
    public static ElementTheme Requested(string choice) => choice switch
    {
        "light" => ElementTheme.Light,
        "dark" => ElementTheme.Dark,
        _ => ElementTheme.Default,
    };

    /// <summary>§4.10: with the dark theme, the background from dark grey (0)
    /// to black (100).</summary>
    public static Color DarkBackground(int darkness)
    {
        var level = (byte)Math.Round(48.0 * (100 - Math.Clamp(darkness, 0, 100)) / 100);
        return Color.FromArgb(255, level, level, level);
    }

    public static SolidColorBrush Solid(Color c) => new(c);
    public static readonly Color Transparent = Colors.Transparent;
}
