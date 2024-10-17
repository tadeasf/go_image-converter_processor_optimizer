import * as fs from 'fs';
import * as path from 'path';

const inputDir: string = '/Users/tadeasfort/Downloads/1_rachel-to-convert-then-crop-then-ups';
const outputDir: string = '/Users/tadeasfort/Downloads/1_rachel-to-convert-then-crop-then-ups/jpg';

function getFilenamesWithExtension(dir: string): string[] {
    return fs.readdirSync(dir)
        .filter((file: string) => {
            const ext: string = path.extname(file).toLowerCase();
            return ['.heic', '.png', '.jpg', '.jpeg'].includes(ext);
        });
}

function removeTimestamp(filename: string): string {
    return filename.replace(/_\d+(\.\w+)$/, '$1');
}

const inputFiles: string[] = getFilenamesWithExtension(inputDir);
const outputFiles: string[] = getFilenamesWithExtension(outputDir).map(removeTimestamp);

const missingFiles: string[] = [];
const matchedFiles: { input: string, output: string }[] = [];

inputFiles.forEach(inputFile => {
    const inputName = path.parse(inputFile).name;
    const matchedOutput = outputFiles.find(outputFile => 
        outputFile.startsWith(inputName) || path.parse(outputFile).name === inputName
    );

    if (matchedOutput) {
        matchedFiles.push({ input: inputFile, output: matchedOutput });
    } else {
        missingFiles.push(inputFile);
    }
});

console.log('Files that failed to convert:');
missingFiles.forEach((file: string) => console.log(file));
console.log(`\nTotal missing files: ${missingFiles.length}`);
console.log(`Input files: ${inputFiles.length}`);
console.log(`Output files: ${outputFiles.length}`);

console.log('\nMatched files (first 10):');
matchedFiles.slice(0, 10).forEach(({ input, output }) => {
    console.log(`${input} -> ${output}`);
});

console.log('\nUnmatched output files:');
const unmatchedOutputs = outputFiles.filter(outputFile => 
    !matchedFiles.some(match => match.output === outputFile)
);
unmatchedOutputs.forEach(file => console.log(file));