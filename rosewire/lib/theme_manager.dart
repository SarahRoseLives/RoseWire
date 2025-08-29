import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

// Static status colors that do not change with the theme
const statusGreen = Color(0xFF26C281);
const statusOrange = Colors.orange;
const statusRed = Colors.redAccent;

/// A simple ChangeNotifier to manage the application's theme.
class ThemeManager extends ChangeNotifier {
  // Default to Rose Pink if no theme is set.
  ThemeData _themeData = createTheme(const Color(0xFFEA4C89));

  /// The current theme data for the application.
  ThemeData get themeData => _themeData;

  /// A list of predefined accent colors for the user to choose from.
  static final List<Color> availableColors = [
    const Color(0xFFEA4C89), // Rose Pink (Default)
    const Color(0xFF3498DB), // Blue
    const Color(0xFF9B59B6), // Purple
    const Color(0xFFF1C40F), // Yellow
    const Color(0xFFE67E22), // Orange
    const Color(0xFFE74C3C), // Red
    const Color(0xFF1ABC9C), // Teal
    Colors.white,           // White
  ];

  /// Creates a ThemeData instance based on a provided accent color.
  static ThemeData createTheme(Color accentColor) {
    return ThemeData(
      brightness: Brightness.dark,
      useMaterial3: true,
      primaryColor: accentColor,
      colorScheme: ColorScheme.dark(
        primary: accentColor,
        // --- CHANGE: Make the secondary color match the primary accent color ---
        secondary: accentColor,
        // --- END CHANGE ---
        background: const Color(0xFF121212),
        surface: const Color(0xFF1E1E1E),
        onPrimary: Colors.black, // Good contrast for bright accent colors
        onSecondary: Colors.black, // Changed to match onPrimary
        onBackground: Colors.white,
        onSurface: Colors.white,
        error: statusRed,
        onError: Colors.white,
      ),
      scaffoldBackgroundColor: const Color(0xFF121212),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: accentColor,
          foregroundColor: Colors.black, // Contrasting text for buttons
        ),
      ),
    );
  }

  /// Loads the saved theme color from shared preferences.
  Future<void> loadTheme() async {
    final prefs = await SharedPreferences.getInstance();
    final colorValue = prefs.getInt('theme_color') ?? availableColors.first.value;
    _themeData = createTheme(Color(colorValue));
    notifyListeners();
  }

  /// Sets the application theme and persists the choice.
  void setTheme(Color accentColor) async {
    if (!availableColors.contains(accentColor)) return;
    _themeData = createTheme(accentColor);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt('theme_color', accentColor.value);
    notifyListeners();
  }
}

// Global instance of the ThemeManager
final themeManager = ThemeManager();