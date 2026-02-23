/* eslint-disable @typescript-eslint/no-var-requires */
const path = require('path');
const { ConsoleRemotePlugin } = require('@openshift-console/dynamic-plugin-sdk-webpack');

module.exports = {
  mode: 'production',
  entry: {},
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: '[name]-bundle.js',
    chunkFilename: '[name]-chunk.js',
  },
  resolve: {
    extensions: ['.ts', '.tsx', '.js', '.jsx', '.json'],
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: {
          loader: 'ts-loader',
          options: {
            configFile: path.resolve(__dirname, 'tsconfig.json'),
          },
        },
        exclude: /node_modules/,
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
      {
        test: /\.(png|jpg|jpeg|gif|svg)$/,
        type: 'asset/resource',
      },
    ],
  },
  plugins: [
    new ConsoleRemotePlugin({ validateSharedModules: false }),
  ],
  externals: {
    react: 'React',
    'react-dom': 'ReactDOM',
    'react-router-dom': 'ReactRouterDOM',
  },
  optimization: {
    minimize: true,
    splitChunks: {
      chunks: 'all',
    },
  },
  devtool: 'source-map',
};
