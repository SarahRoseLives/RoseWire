import 'dart:io';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:path_provider/path_provider.dart';
import '../../../services/ssh_chat_service.dart';

// Helper function to get an icon based on the file extension.
IconData _getIconForFile(String fileName) {
  final extension = fileName.contains('.') ? fileName.split('.').last.toLowerCase() : '';
  switch (extension) {
    // Audio
    case 'mp3':
    case 'wav':
    case 'aac':
    case 'flac':
    case 'ogg':
    case 'm4a':
      return Icons.music_note;
    // Video
    case 'mp4':
    case 'mov':
    case 'avi':
    case 'mkv':
    case 'webm':
      return Icons.movie;
    // Image
    case 'jpg':
    case 'jpeg':
    case 'png':
    case 'gif':
    case 'bmp':
    case 'webp':
      return Icons.image;
    // Archive
    case 'zip':
    case 'rar':
    case '7z':
    case 'tar':
    case 'gz':
      return Icons.archive;
    // Document
    case 'pdf':
      return Icons.picture_as_pdf;
    case 'doc':
    case 'docx':
      return Icons.description; // Generic document icon
    case 'xls':
    case 'xlsx':
      return Icons.grid_on; // Spreadsheet icon
    case 'ppt':
    case 'pptx':
      return Icons.slideshow; // Presentation icon
    // Code/Text
    case 'txt':
    case 'md':
    case 'log':
      return Icons.article;
    case 'json':
    case 'xml':
    case 'html':
    case 'css':
    case 'js':
    case 'dart':
    case 'py':
    case 'java':
    case 'c':
    case 'cpp':
    case 'sh':
      return Icons.code;
    // Default
    default:
      return Icons.insert_drive_file;
  }
}


class LibraryPanelMobile extends StatefulWidget {
  final String nickname;
  final SshChatService chatService;
  final String? initialPath;
  final void Function(String folderPath, List<Map<String, dynamic>> files) onLibraryChanged;

  const LibraryPanelMobile({
    super.key,
    required this.nickname,
    required this.chatService,
    required this.onLibraryChanged,
    this.initialPath,
  });

  @override
  State<LibraryPanelMobile> createState() => _LibraryPanelMobileState();
}

class _LibraryPanelMobileState extends State<LibraryPanelMobile> {
  List<File> _files = [];
  bool _loading = false;
  String? _error;
  String? _libraryPath;

  String get _configFilename => "${widget.nickname}_rosewire_library.json";

  @override
  void initState() {
    super.initState();
    // If an initial path was provided by the main app, load files from it.
    if (widget.initialPath != null) {
      _loadFilesFromFolder(widget.initialPath!, persist: false);
    }
  }

  Future<File> _getLibraryConfigFile() async {
    final dir = await getApplicationSupportDirectory();
    return File('${dir.path}/$_configFilename');
  }

  Future<void> _selectFolder() async {
    String? selectedPath = await FilePicker.platform.getDirectoryPath(
      dialogTitle: 'Please select your RoseWire library folder',
    );

    if (selectedPath != null && selectedPath.isNotEmpty) {
      await _loadFilesFromFolder(selectedPath, persist: true);
    }
  }

  Future<void> _loadFilesFromFolder(String folderPath, {bool persist = true}) async {
    if (mounted) {
      setState(() {
        _libraryPath = folderPath;
        _loading = true;
        _error = null;
      });
    }

    try {
      final dir = Directory(folderPath);
      if (await dir.exists()) {
        final files = await dir.list().where((f) => f is File).cast<File>().toList();

        if (mounted) {
          setState(() {
            _files = files;
            _loading = false;
          });
        }

        if (persist) {
          final configFile = await _getLibraryConfigFile();
          final config = {"folderPath": folderPath};
          await configFile.writeAsString(jsonEncode(config));
        }

        final filesPayload = files.map((file) {
          return {
            "Name": file.path.split('/').last,
            "Size": file.lengthSync(),
            "IsDir": false,
          };
        }).toList();

        widget.onLibraryChanged(folderPath, filesPayload);

      } else {
        if (mounted) {
          setState(() {
            _error = "Selected directory does not exist.";
            _loading = false;
          });
        }
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = "Failed to load files. Please check permissions.";
          _loading = false;
        });
      }
      print("Error loading files: $e");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.grey[900],
      body: Column(
        children: [
          _buildHeader(),
          Expanded(child: _buildBody()),
        ],
      ),
    );
  }

  Widget _buildHeader() {
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            "My Shared Library",
            style: TextStyle(
              fontSize: 20,
              color: Colors.white,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            _libraryPath ?? "No folder selected",
            style: TextStyle(color: Colors.white.withOpacity(0.7), fontSize: 14),
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton.icon(
              icon: const Icon(Icons.folder_open),
              label: const Text("Select Library Folder"),
              onPressed: _selectFolder,
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.pinkAccent,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 12),
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(child: Text(_error!, style: const TextStyle(color: Colors.redAccent)));
    }
    if (_libraryPath == null) {
      return const Center(
        child: Text("Please select a folder to share your files.", style: TextStyle(color: Colors.white70)),
      );
    }
    if (_files.isEmpty) {
      return const Center(
        child: Text("No files found in the selected folder.", style: TextStyle(color: Colors.white70)),
      );
    }

    return ListView.builder(
      itemCount: _files.length,
      itemBuilder: (context, idx) {
        final file = _files[idx];
        final name = file.path.split(Platform.pathSeparator).last;
        final size = file.lengthSync();
        final formattedSize = size < 1024 ? "$size B" :
                              size < 1024 * 1024 ? "${(size / 1024).toStringAsFixed(2)} KB" :
                              "${(size / (1024 * 1024)).toStringAsFixed(2)} MB";

        return Card(
          margin: const EdgeInsets.symmetric(vertical: 4, horizontal: 16),
          color: Colors.grey[850],
          child: ListTile(
            leading: Icon(_getIconForFile(name), color: Colors.pinkAccent),
            title: Text(name, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold)),
            subtitle: Text(
              formattedSize,
              style: TextStyle(color: Colors.white.withOpacity(0.7)),
            ),
          ),
        );
      },
    );
  }
}